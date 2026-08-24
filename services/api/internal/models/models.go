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

// Deployment is the primary, agency/contract-level record: what FlockWatch's
// "Public Records Atlas" shows by default, at city-level precision.
type Deployment struct {
	ID               string           `json:"id"`
	AgencyName       string           `json:"agency_name"`
	City             string           `json:"city"`
	State            string           `json:"state"`
	County           *string          `json:"county,omitempty"`
	Lat              *float64         `json:"lat,omitempty"`
	Lng              *float64         `json:"lng,omitempty"`
	DocumentedUnits  *int             `json:"documented_units,omitempty"`
	EvidenceType     EvidenceType     `json:"evidence_type"`
	SourceLinks      []string         `json:"source_links"`
	Status           DeploymentStatus `json:"status"`
	Notes            *string          `json:"notes,omitempty"`
	CreatedBy        *string          `json:"created_by,omitempty"`
	ReviewedBy       *string          `json:"reviewed_by,omitempty"`
	LastReviewedAt   *time.Time       `json:"last_reviewed_at,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
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
	PhotoURL     *string   `json:"photo_url,omitempty"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	CreatedBy    *string   `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
