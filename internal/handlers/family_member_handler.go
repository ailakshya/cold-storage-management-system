package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/gorilla/mux"
)

type FamilyMemberHandler struct {
	Repo *repositories.FamilyMemberRepository
}

func NewFamilyMemberHandler(repo *repositories.FamilyMemberRepository) *FamilyMemberHandler {
	return &FamilyMemberHandler{Repo: repo}
}

// List returns all family members for a customer
func (h *FamilyMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	customerIDStr := mux.Vars(r)["id"]
	customerID, err := strconv.Atoi(customerIDStr)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	members, err := h.Repo.ListByCustomer(r.Context(), customerID)
	if err != nil {
		http.Error(w, "Failed to fetch family members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if members == nil {
		members = []models.FamilyMember{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"family_members": members,
		"relations":      models.FamilyMemberRelations,
	})
}

// Create creates a new family member for a customer
func (h *FamilyMemberHandler) Create(w http.ResponseWriter, r *http.Request) {
	customerIDStr := mux.Vars(r)["id"]
	customerID, err := strconv.Atoi(customerIDStr)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	var req models.CreateFamilyMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if req.Relation == "" {
		req.Relation = "Other"
	}

	// Check if name already exists for this customer
	existing, _ := h.Repo.GetByCustomerAndName(r.Context(), customerID, req.Name)
	if existing != nil {
		http.Error(w, "Family member with this name already exists", http.StatusConflict)
		return
	}

	member := &models.FamilyMember{
		CustomerID: customerID,
		Name:       req.Name,
		Relation:   req.Relation,
		IsDefault:  req.Relation == "Self",
	}

	if err := h.Repo.Create(r.Context(), member); err != nil {
		http.Error(w, "Failed to create family member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(member)
}

// Update updates a family member
func (h *FamilyMemberHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid family member ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateFamilyMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	member, err := h.Repo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "Family member not found", http.StatusNotFound)
		return
	}

	if req.Name != "" {
		member.Name = req.Name
	}
	if req.Relation != "" {
		member.Relation = req.Relation
	}

	if err := h.Repo.Update(r.Context(), member); err != nil {
		http.Error(w, "Failed to update family member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(member)
}

// Delete deletes a family member
func (h *FamilyMemberHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid family member ID", http.StatusBadRequest)
		return
	}

	member, err := h.Repo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "Family member not found", http.StatusNotFound)
		return
	}

	// Don't allow deleting the default (Self) member if it has entries
	if member.IsDefault && member.EntryCount > 0 {
		http.Error(w, "Cannot delete default family member with entries", http.StatusBadRequest)
		return
	}

	if err := h.Repo.Delete(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete family member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Family member deleted successfully",
	})
}

// GetRelations returns the list of available relations
func (h *FamilyMemberHandler) GetRelations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"relations": models.FamilyMemberRelations,
	})
}
