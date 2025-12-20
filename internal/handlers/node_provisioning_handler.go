package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

// NodeProvisioningHandler handles node provisioning API endpoints
type NodeProvisioningHandler struct {
	Service *services.NodeProvisioningService
}

// NewNodeProvisioningHandler creates a new node provisioning handler
func NewNodeProvisioningHandler(service *services.NodeProvisioningService) *NodeProvisioningHandler {
	return &NodeProvisioningHandler{Service: service}
}

// ListNodes returns all cluster nodes
func (h *NodeProvisioningHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.Service.GetNodes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// GetNode returns a single node by ID
func (h *NodeProvisioningHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	node, err := h.Service.GetNode(r.Context(), id)
	if err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// AddNode adds a new node to the cluster
func (h *NodeProvisioningHandler) AddNode(w http.ResponseWriter, r *http.Request) {
	var req models.AddNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := getUserIDFromContext(r)

	node, err := h.Service.AddNode(r.Context(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// TestConnection tests SSH connectivity to a node
func (h *NodeProvisioningHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IPAddress string `json:"ip_address"`
		SSHUser   string `json:"ssh_user"`
		SSHPort   int    `json:"ssh_port"`
		SSHKey    string `json:"ssh_key,omitempty"`
		Password  string `json:"password,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.Service.TestNodeConnection(r.Context(), req.IPAddress, req.SSHUser, req.SSHPort, req.SSHKey, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

// ProvisionNode starts provisioning a node
func (h *NodeProvisioningHandler) ProvisionNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	var req struct {
		SSHKey   string `json:"ssh_key,omitempty"`
		Password string `json:"password,omitempty"`
	}

	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	userID := getUserIDFromContext(r)

	// Start provisioning in background
	go h.Service.ProvisionNode(r.Context(), id, req.SSHKey, req.Password, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Provisioning started",
		"node_id": id,
	})
}

// GetProvisionStatus returns the current provisioning status
func (h *NodeProvisioningHandler) GetProvisionStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	status, active := h.Service.GetProvisionStatus(id)
	if !active {
		// Get node status from database
		node, err := h.Service.GetNode(r.Context(), id)
		if err != nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active":  false,
			"status":  node.Status,
			"message": node.ErrorMessage,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active": true,
		"status": status,
	})
}

// GetProvisionLogs returns provisioning logs for a node
func (h *NodeProvisioningHandler) GetProvisionLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	logs, err := h.Service.GetProvisionLogs(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// RemoveNode removes a node from the cluster
func (h *NodeProvisioningHandler) RemoveNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	userID := getUserIDFromContext(r)

	if err := h.Service.RemoveNode(r.Context(), id, force, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Node removed successfully"})
}

// RebootNode reboots a node
func (h *NodeProvisioningHandler) RebootNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r)

	if err := h.Service.RebootNode(r.Context(), id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Node reboot initiated"})
}

// DrainNode drains pods from a node
func (h *NodeProvisioningHandler) DrainNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r)

	if err := h.Service.DrainNode(r.Context(), id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Node drained successfully"})
}

// CordonNode cordons a node
func (h *NodeProvisioningHandler) CordonNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r)

	if err := h.Service.CordonNode(r.Context(), id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Node cordoned"})
}

// UncordonNode uncordons a node
func (h *NodeProvisioningHandler) UncordonNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r)

	if err := h.Service.UncordonNode(r.Context(), id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Node uncordoned"})
}

// GetNodeLogs returns system logs from a node
func (h *NodeProvisioningHandler) GetNodeLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	lines := 100
	if linesStr := r.URL.Query().Get("lines"); linesStr != "" {
		if l, err := strconv.Atoi(linesStr); err == nil {
			lines = l
		}
	}

	logs, err := h.Service.GetNodeLogs(r.Context(), id, lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"logs": logs})
}

// ListConfigs returns all infrastructure configurations
func (h *NodeProvisioningHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.Service.GetConfigs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

// UpdateConfig updates an infrastructure configuration
func (h *NodeProvisioningHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req models.ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r)

	if err := h.Service.UpdateConfig(r.Context(), req.Key, req.Value, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Configuration updated"})
}

// helper to get user ID from request context
func getUserIDFromContext(r *http.Request) int {
	if claims, ok := r.Context().Value("claims").(map[string]interface{}); ok {
		if userID, ok := claims["user_id"].(float64); ok {
			return int(userID)
		}
	}
	return 0
}
