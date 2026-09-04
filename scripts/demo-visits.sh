#!/bin/sh

set -eu

BASE_URL=${BASE_URL:-http://localhost:8080}
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

require_command() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "error: required command not found: $1" >&2
		exit 1
	}
}

request() {
	method=$1
	path=$2
	body=$3
	expected_status=$4
	response_file=$TMP_DIR/response.json
	status=$(curl -sS -o "$response_file" -w '%{http_code}' \
		-X "$method" -H 'Content-Type: application/json' \
		--data "$body" "$BASE_URL$path") || {
		echo "error: request failed: $method $path" >&2
		exit 1
	}

	if [ "$status" != "$expected_status" ]; then
		echo "error: $method $path returned HTTP $status (expected $expected_status)" >&2
		cat "$response_file" >&2
		exit 1
	fi

	cat "$response_file"
}

request_capture() {
	method=$1
	path=$2
	body=$3
	response_file=$4
	status_file=$5
	status=$(curl -sS -o "$response_file" -w '%{http_code}' \
		-X "$method" -H 'Content-Type: application/json' \
		--data "$body" "$BASE_URL$path") || return 1
	printf '%s' "$status" >"$status_file"
}

require_command curl
require_command jq

doctor=$(request POST /v1/doctors/search '{}' 200)
doctor_id=$(printf '%s' "$doctor" | jq -er '.data[0].id') || {
		echo 'error: doctor search returned no usable doctor ID' >&2
		exit 1
	}
clinic_id=$(printf '%s' "$doctor" | jq -er '.data[0].clinic_id') || {
		echo 'error: doctor search returned no usable clinic ID' >&2
		exit 1
	}

patients=$(request POST '/v1/patients/search?per_page=2' '{}' 200)
patient_ids=$(printf '%s' "$patients" | jq -er '[.data[0].id, .data[1].id] | if any(.[]; . == null or . == "") then error("missing patient ID") else .[] end') || {
		echo 'error: patient search returned fewer than two usable patient IDs' >&2
		exit 1
	}
patient_one=$(printf '%s\n' "$patient_ids" | sed -n '1p')
patient_two=$(printf '%s\n' "$patient_ids" | sed -n '2p')

format_epoch() {
	date -u -d "@$1" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null ||
	date -u -r "$1" '+%Y-%m-%dT%H:%M:%SZ'
}

base_epoch=$(( $(date -u '+%s') + 315360000 ))

visit_one_id=''
visit_two_id=''
attempt=0
	while [ "$attempt" -lt 48 ]; do
	visit_start=$(format_epoch $((base_epoch + attempt * 7200)))
	visit_middle=$(format_epoch $((base_epoch + attempt * 7200 + 3600)))
	visit_end=$(format_epoch $((base_epoch + attempt * 7200 + 7200)))

			first_response=$TMP_DIR/first-response.json
			first_status=$TMP_DIR/first-status
			request_capture POST /v1/visits/create "$(printf '{"doctor_id":"%s","patient_id":"%s","clinic_id":"%s","visit_start_time":"%s","visit_end_time":"%s"}' \
				"$doctor_id" "$patient_one" "$clinic_id" "$visit_start" "$visit_middle")" "$first_response" "$first_status" || {
				echo 'error: first visit request failed' >&2
				exit 1
			}
			case "$(cat "$first_status")" in
				201)
					visit_one_id=$(jq -er '.data.id' <"$first_response") || {
						echo 'error: first visit response did not contain an ID' >&2
						exit 1
					}
					;;
				409)
					attempt=$((attempt + 1))
					continue
					;;
				*)
					echo "error: first visit returned HTTP $(cat "$first_status")" >&2
					cat "$first_response" >&2
					exit 1
					;;
			esac

		second_response=$TMP_DIR/second-response.json
		second_status=$TMP_DIR/second-status
		request_capture POST /v1/visits/create "$(printf '{"doctor_id":"%s","patient_id":"%s","clinic_id":"%s","visit_start_time":"%s","visit_end_time":"%s"}' \
			"$doctor_id" "$patient_two" "$clinic_id" "$visit_middle" "$visit_end")" "$second_response" "$second_status" || {
			echo 'error: second visit request failed' >&2
			exit 1
		}
		case "$(cat "$second_status")" in
			201)
				visit_two_id=$(jq -er '.data.id' <"$second_response") || {
					echo 'error: second visit response did not contain an ID' >&2
					exit 1
				}
				break
				;;
			409)
				request DELETE /v1/visits/delete "$(printf '{"visit_id":"%s"}' "$visit_one_id")" 204 >/dev/null
				visit_one_id=''
				;;
			*)
				echo "error: second visit returned HTTP $(cat "$second_status")" >&2
				cat "$second_response" >&2
				exit 1
				;;
		esac
	attempt=$((attempt + 1))
done

if [ -z "$visit_one_id" ] || [ -z "$visit_two_id" ]; then
	echo 'error: could not find two adjacent, non-conflicting future visit slots after 48 attempts' >&2
	exit 1
fi

echo "created visit: $visit_one_id"
echo "created visit: $visit_two_id"

page=1
found_first=false
found_second=false
while [ "$page" -le 1000 ] && { [ "$found_first" != true ] || [ "$found_second" != true ]; }; do
	listed=$(request POST "/v1/visits/list?page=$page&per_page=200" '{}' 200)
	printf '%s' "$listed" | jq -e --arg id "$visit_one_id" '[.data[].id] | index($id) != null' >/dev/null && found_first=true || :
	printf '%s' "$listed" | jq -e --arg id "$visit_two_id" '[.data[].id] | index($id) != null' >/dev/null && found_second=true || :
	count=$(printf '%s' "$listed" | jq -er '.data | length')
	[ "$count" -lt 200 ] && break
	page=$((page + 1))
done
if [ "$found_first" != true ] || [ "$found_second" != true ]; then
	echo 'error: paginated list did not contain both created visits' >&2
	exit 1
fi
echo 'listed both created visits'

updated=$(request PATCH /v1/visits/update "{\"visit_id\":\"$visit_one_id\",\"status\":\"IN_PROGRESS\"}" 200)
printf '%s' "$updated" | jq -e --arg id "$visit_one_id" \
	'.data.id == $id and .data.status == "IN_PROGRESS"' >/dev/null || {
	echo 'error: update response did not confirm IN_PROGRESS' >&2
	exit 1
}
echo "updated visit $visit_one_id to IN_PROGRESS"
