package domain

// LegalDetails holds the registry (ЕГРЮЛ/ЕГРИП) facts for a company, looked up
// by an Enricher (Phase 3, #188) from an official source (DaData). All fields
// are plain strings already validated at the point of construction (INN/OGRN
// pass their VO checksums before they reach here). Stored inside CompanyProfile
// as a single JSONB sub-object, so adding it is backward compatible.
type LegalDetails struct {
	INN      string `json:"inn,omitempty"`
	OGRN     string `json:"ogrn,omitempty"`
	FullName string `json:"fullName,omitempty"` // official name with legal form
	Address  string `json:"address,omitempty"`
	OKVED    string `json:"okved,omitempty"` // primary activity code
	Status   string `json:"status,omitempty"` // ACTIVE / LIQUIDATING / ...
}

// IsEmpty reports whether no registry data was found.
func (d LegalDetails) IsEmpty() bool {
	return d.INN == "" && d.OGRN == "" && d.FullName == "" &&
		d.Address == "" && d.OKVED == "" && d.Status == ""
}
