package doctor

type Doctor struct {
	ID          string `json:"id"`
	SpecialtyID string `json:"specialty_id"`
	ClinicID    string `json:"clinic_id"`
	FullName    string `json:"full_name"`
}
