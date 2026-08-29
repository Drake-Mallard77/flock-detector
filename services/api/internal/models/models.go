package models

import "time"

type Role string

const (
	RoleSubmitter Role = "submitter"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

type DeploymentStatus string

const (
	StatusConfirmed     DeploymentStatus = "confirmed"
	StatusContractFound DeploymentStatus = "contract_found"
	// Published, but explicitly NOT verified against public records:
	// mapped by OpenStreetMap contributors and attributed to this operator.
	// Distinct from StatusConfirmed so the atlas never claims a level of
	// verification it hasn't done. See migration 0006.
	StatusOSMDocumented DeploymentStatus = "osm_documented"
	StatusUnderReview   DeploymentStatus = "under_review"
	StatusDisputed      DeploymentStatus = "disputed"
	StatusRemoved       DeploymentStatus = "removed"
)

type EvidenceType string

const (
	EvidenceCouncilReport EvidenceType = "council_report"
	EvidenceContract      EvidenceType = "contract"
	EvidenceInvoice       EvidenceType = "invoice"
	EvidenceNewsArticle   EvidenceType = "news_article"
	EvidenceFOIAResponse  EvidenceType = "foia_response"
	EvidenceUserPhoto     EvidenceType = "user_photo"
	EvidenceOSMImport     EvidenceType = "osm_import"
)

// ValidEvidenceTypes lists every evidence_type value the deployments table's
// CHECK constraint (migrations/0001_init.sql) accepts. Handlers validate
// against this before hitting the DB so a bad value comes back as a 400 with
// a clear message instead of a generic 500 from the constraint violation.
var ValidEvidenceTypes = []EvidenceType{
	EvidenceCouncilReport, EvidenceContract, EvidenceInvoice,
	EvidenceNewsArticle, EvidenceFOIAResponse, EvidenceUserPhoto, EvidenceOSMImport,
}

func IsValidEvidenceType(v string) bool {
	for _, e := range ValidEvidenceTypes {
		if string(e) == v {
			return true
		}
	}
	return false
}

// ValidDeploymentStatuses lists every status value the deployments table's
// CHECK constraint accepts.
var ValidDeploymentStatuses = []DeploymentStatus{
	StatusConfirmed, StatusContractFound, StatusOSMDocumented,
	StatusUnderReview, StatusDisputed, StatusRemoved,
}

func IsValidDeploymentStatus(v string) bool {
	for _, s := range ValidDeploymentStatuses {
		if string(s) == v {
			return true
		}
	}
	return false
}

// Deployment is the primary, agency/contract-level record: what FlockWatch's
// "Public Records Atlas" shows by default, at city-level precision.
type Deployment struct {
	ID         string `json:"id"`
	AgencyName string `json:"agency_name"`
	// Slug is the record's readable URL segment, unique within its state.
	// Sent with every record so a list can link to canonical URLs without a
	// second lookup per row.
	Slug            string       `json:"slug"`
	City            string       `json:"city"`
	State           string       `json:"state"`
	County          *string      `json:"county,omitempty"`
	Lat             *float64     `json:"lat,omitempty"`
	Lng             *float64     `json:"lng,omitempty"`
	DocumentedUnits *int         `json:"documented_units,omitempty"`
	EvidenceType    EvidenceType `json:"evidence_type"`
	// What kind of body operates this deployment, when known. Lets the
	// atlas distinguish a police department from a retailer rather than
	// presenting both as the same kind of claim.
	OperatorType   *string          `json:"operator_type,omitempty"`
	SourceLinks    []string         `json:"source_links"`
	Status         DeploymentStatus `json:"status"`
	Notes          *string          `json:"notes,omitempty"`
	CreatedBy      *string          `json:"created_by,omitempty"`
	ReviewedBy     *string          `json:"reviewed_by,omitempty"`
	LastReviewedAt *time.Time       `json:"last_reviewed_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// CameraSighting is the opt-in, precise-pin layer: exact camera locations
// from user submissions or the OSM/DeFlock bootstrap import.
type CameraSighting struct {
	ID           string    `json:"id"`
	DeploymentID *string   `json:"deployment_id,omitempty"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	Direction    *int      `json:"direction,omitempty"`
	CameraType   *string   `json:"camera_type,omitempty"`
	Manufacturer *string   `json:"manufacturer,omitempty"`
	PhotoURL     *string   `json:"photo_url,omitempty"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	ExternalID   *string   `json:"external_id,omitempty"`
	State        *string   `json:"state,omitempty"`
	CreatedBy    *string   `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
