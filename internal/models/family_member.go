package models

import "time"

// FamilyMember represents a family member associated with a customer
// Allows one phone number to have multiple people storing items
type FamilyMember struct {
	ID         int       `json:"id"`
	CustomerID int       `json:"customer_id"`
	Name       string    `json:"name"`
	Relation   string    `json:"relation"`   // Self, Son, Daughter, Brother, Sister, Father, Mother, Wife, Husband, Partner, Other
	IsDefault  bool      `json:"is_default"` // True for "Self" member
	EntryCount int       `json:"entry_count,omitempty"` // Number of entries for this family member
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Relation options for family members
var FamilyMemberRelations = []string{
	"Self",
	"Son",
	"Daughter",
	"Brother",
	"Sister",
	"Father",
	"Mother",
	"Wife",
	"Husband",
	"Partner",
	"Other",
}

// CreateFamilyMemberRequest for creating a new family member
type CreateFamilyMemberRequest struct {
	Name     string `json:"name"`
	Relation string `json:"relation"`
}

// UpdateFamilyMemberRequest for updating a family member
type UpdateFamilyMemberRequest struct {
	Name     string `json:"name"`
	Relation string `json:"relation"`
}
