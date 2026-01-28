package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"cold-backend/internal/middleware"
	"cold-backend/internal/services"
)

type FileManagerHandler struct {
	UserService *services.UserService
	TOTPService *services.TOTPService
	RootPaths   map[string]string
}

func NewFileManagerHandler(userService *services.UserService, totpService *services.TOTPService, backupDir string) *FileManagerHandler {
	// Default paths
	paths := map[string]string{
		"bulk":      "/mass-pool/shared",
		"highspeed": "/fast-pool/data",
		"archives":  "/mass-pool/archives",
		"backups":   "/mass-pool/backups",
		"trash":     "/mass-pool/trash",
	}

	// Update backups path from config
	if backupDir != "" {
		paths["backups"] = backupDir
	}

	// If mass-pool doesn't exist (Dev environment), remap other paths to local temp/home
	if _, err := os.Stat("/mass-pool"); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		base := filepath.Join(home, "cold-storage")

		// Ensure base exists
		os.MkdirAll(base, 0755)

		paths["bulk"] = filepath.Join(base, "shared")
		paths["highspeed"] = filepath.Join(base, "data")
		paths["archives"] = filepath.Join(base, "archives")
		paths["trash"] = filepath.Join(base, "trash")

		// Create directories and LOG mappings for debugging
		for k, p := range paths {
			if err := os.MkdirAll(p, 0755); err != nil {
				log.Printf("[FileManager] Error creating root %s at %s: %v", k, p, err)
			} else {
				log.Printf("[FileManager] Root %s mapped to: %s", k, p)
			}
		}

	}

	return &FileManagerHandler{
		UserService: userService,
		TOTPService: totpService,
		RootPaths:   paths,
	}
}

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Type    string    `json:"type"` // "dir", "file", "image", "pdf", etc.
}

type ListResponse struct {
	Files       []FileInfo `json:"files"`
	CurrentPath string     `json:"current_path"`
}

type StorageStats struct {
	Root        string  `json:"root"`
	Label       string  `json:"label"`
	Path        string  `json:"path"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// GetStorageStats returns filesystem usage for all storage pools
func (h *FileManagerHandler) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	labels := map[string]string{
		"bulk":      "Bulk Storage",
		"highspeed": "High Speed",
		"archives":  "Archives",
		"backups":   "Backups",
		"trash":     "Trash",
	}

	stats := []StorageStats{}

	for key, path := range h.RootPaths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			// If path doesn't exist or error, skip
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bfree * uint64(stat.Bsize)
		usedBytes := totalBytes - freeBytes
		usedPercent := 0.0
		if totalBytes > 0 {
			usedPercent = float64(usedBytes) / float64(totalBytes) * 100
		}

		stats = append(stats, StorageStats{
			Root:        key,
			Label:       labels[key],
			Path:        path,
			TotalBytes:  totalBytes,
			UsedBytes:   usedBytes,
			FreeBytes:   freeBytes,
			UsedPercent: usedPercent,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// resolvePath validates and resolves the full path
func (h *FileManagerHandler) resolvePath(rootKey, subPath string) (string, error) {
	baseRoot, ok := h.RootPaths[rootKey]
	if !ok {
		return "", fmt.Errorf("invalid root key")
	}

	// Clean the subpath to prevent directory traversal
	cleanSubPath := filepath.Clean("/" + subPath)
	if strings.Contains(cleanSubPath, "..") {
		return "", fmt.Errorf("invalid path")
	}

	fullPath := filepath.Join(baseRoot, cleanSubPath)

	// Ensure the resolved path is still within the allowed root
	if !strings.HasPrefix(fullPath, baseRoot) {
		return "", fmt.Errorf("path escapes root")
	}

	return fullPath, nil
}

// ListFiles lists files in a directory
func (h *FileManagerHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	rootKey := r.URL.Query().Get("root")
	subPath := r.URL.Query().Get("path")
	searchQuery := strings.ToLower(r.URL.Query().Get("search"))

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	files := []FileInfo{}

	if searchQuery != "" {
		baseRoot, ok := h.RootPaths[rootKey]
		if !ok {
			http.Error(w, "Invalid root", http.StatusInternalServerError)
			return
		}

		getFileType := func(isDir bool, name string) string {
			if isDir {
				return "dir"
			}
			ext := strings.ToLower(filepath.Ext(name))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
				return "image"
			case ".pdf":
				return "pdf"
			case ".mp4", ".mov", ".avi", ".webm":
				return "video"
			case ".zip", ".tar", ".gz", ".rar":
				return "archive"
			case ".txt", ".md", ".log":
				return "text"
			default:
				return "file"
			}
		}

		err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == fullPath {
				return nil
			}

			if strings.Contains(strings.ToLower(d.Name()), searchQuery) {
				relPath, _ := filepath.Rel(baseRoot, path)
				info, err := d.Info()
				if err != nil {
					return nil
				}

				var size int64 = info.Size()
				if d.IsDir() {
					size, _ = getDirSize(path)
				}

				files = append(files, FileInfo{
					Name:    d.Name(),
					Path:    relPath,
					IsDir:   d.IsDir(),
					Size:    size,
					ModTime: info.ModTime(),
					Type:    getFileType(d.IsDir(), d.Name()),
				})

				if len(files) >= 200 {
					return fmt.Errorf("limit_reached")
				}
			}
			return nil
		})

		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return files[i].Name < files[j].Name
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListResponse{
			Files:       files,
			CurrentPath: subPath,
		})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(subPath, entry.Name())
		fileType := "file"
		if entry.IsDir() {
			fileType = "dir"
		} else {
			// Detect file type by extension
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
				fileType = "image"
			case ".pdf":
				fileType = "pdf"
			case ".mp4", ".mov", ".avi", ".webm":
				fileType = "video"
			case ".zip", ".tar", ".gz", ".rar":
				fileType = "archive"
			case ".txt", ".md", ".log":
				fileType = "text"
			}
		}

		var size int64 = info.Size()
		if entry.IsDir() {
			size, _ = getDirSize(filepath.Join(fullPath, entry.Name()))
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    relPath,
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: info.ModTime(),
			Type:    fileType,
		})
	}

	// Sort: directories first, then files
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListResponse{
		Files:       files,
		CurrentPath: subPath,
	})
}

// UploadFile handles file upload with automatic directory creation
func (h *FileManagerHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// maximize upload size (e.g. 10GB allowed - handled by streaming usually, strictly limit in prod)
	r.ParseMultipartForm(10 << 30) // 10 GB limit for form parsing

	rootKey := r.FormValue("root")
	subPath := r.FormValue("path")

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Create all parent directories if they don't exist
	if err := os.MkdirAll(fullPath, 0777); err != nil {
		http.Error(w, "Error creating directories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	destPath := filepath.Join(fullPath, header.Filename)
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error saving file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dst.Close()

	// Convert video using helper
	finalPath, err := convertVideoIfNeeded(destPath)
	if err != nil {
		fmt.Printf("Video conversion failed for %s: %v\n", header.Filename, err)
	}

	// Set permissions
	os.Chmod(finalPath, 0666)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "file": filepath.Base(finalPath)})
}

// UploadChunk handles chunked file uploads
func (h *FileManagerHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (50MB limit)
	r.ParseMultipartForm(50 << 20)

	uploadID := r.FormValue("uploadId")
	chunkIndexStr := r.FormValue("chunkIndex")
	totalChunksStr := r.FormValue("totalChunks")
	filename := r.FormValue("filename")
	rootKey := r.FormValue("root")
	subPath := r.FormValue("path")

	if uploadID == "" || chunkIndexStr == "" || totalChunksStr == "" || filename == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		http.Error(w, "Invalid total chunks", http.StatusBadRequest)
		return
	}

	// Resolve destination directory
	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Temp directory for chunks: /tmp/cold_uploads/<uploadID>
	tempDir := filepath.Join("/tmp", "cold_uploads", uploadID)
	if err := os.MkdirAll(tempDir, 0777); err != nil {
		http.Error(w, "Failed to create temp dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Save chunk to temp file
	file, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "Error retrieving chunk: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d", chunkIndex))
	dst, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, "Failed to create chunk file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save chunk: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if all chunks are uploaded
	// Simple check: count files in tempDir
	entries, err := os.ReadDir(tempDir)
	if err == nil && len(entries) == totalChunks {
		// All chunks received, assemble file
		finalDestPath := filepath.Join(fullPath, filepath.Base(filename))

		// Create final file
		finalFile, err := os.Create(finalDestPath)
		if err != nil {
			http.Error(w, "Failed to create final file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer finalFile.Close()

		// Append chunks in order
		for i := 0; i < totalChunks; i++ {
			chunkPartPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d", i))
			chunkData, err := os.ReadFile(chunkPartPath)
			if err != nil {
				// Cleanup and fail
				finalFile.Close()
				os.Remove(finalDestPath)
				http.Error(w, "Failed to read chunk "+strconv.Itoa(i), http.StatusInternalServerError)
				return
			}
			if _, err := finalFile.Write(chunkData); err != nil {
				finalFile.Close()
				os.Remove(finalDestPath)
				http.Error(w, "Failed to write to final file", http.StatusInternalServerError)
				return
			}
			os.Remove(chunkPartPath) // Delete chunk after append
		}
		finalFile.Close()     // Explicit close before removal/conversion
		os.RemoveAll(tempDir) // Cleanup temp dir

		// Helper function call for video conversion
		convertedPath, err := convertVideoIfNeeded(finalDestPath)
		if err != nil {
			fmt.Printf("Video conversion failed: %v\n", err)
		}

		// Perms
		os.Chmod(convertedPath, 0666)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "completed",
			"file":   filepath.Base(convertedPath),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "chunk_uploaded"})
}

// convertVideoIfNeeded checks if file is video and converts to MOV, removing original
func convertVideoIfNeeded(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".webm" || ext == ".flv" || ext == ".wmv" {
		movPath := strings.TrimSuffix(path, ext) + ".mov"
		// -threads 0 uses all available cores
		cmd := exec.Command("ffmpeg", "-i", path, "-c:v", "libx264", "-c:a", "aac", "-threads", "0", "-y", movPath)
		if err := cmd.Run(); err != nil {
			return path, err
		}
		// Success
		os.Remove(path)
		return movPath, nil
	}
	return path, nil
}

// RenameFile renames a file or directory
func (h *FileManagerHandler) RenameFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Root    string `json:"root"`
		OldPath string `json:"oldPath"`
		NewName string `json:"newName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	oldFullPath, err := h.resolvePath(req.Root, req.OldPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Validate NewName (should be just a name, no path separators)
	if strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "\\") || req.NewName == ".." || req.NewName == "." {
		http.Error(w, "Invalid file name", http.StatusBadRequest)
		return
	}

	parentDir := filepath.Dir(oldFullPath)
	newFullPath := filepath.Join(parentDir, req.NewName)

	// Verify new path is within root
	baseRoot, ok := h.RootPaths[req.Root]
	if !ok || !strings.HasPrefix(newFullPath, baseRoot) {
		http.Error(w, "Invalid destination path", http.StatusForbidden)
		return
	}

	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		http.Error(w, "Failed to rename: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// DownloadFile serves a file for download or inline viewing
func (h *FileManagerHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	rootKey := r.URL.Query().Get("root")
	subPath := r.URL.Query().Get("path")
	mode := r.URL.Query().Get("mode") // "inline" or "attachment"

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		http.Error(w, "Cannot download directory", http.StatusBadRequest)
		return
	}

	disposition := "attachment"
	if mode == "inline" {
		disposition = "inline"
	}

	// Set proper MIME type for video files (critical for Safari)
	ext := strings.ToLower(filepath.Ext(fullPath))
	switch ext {
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".mov":
		w.Header().Set("Content-Type", "video/quicktime")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	case ".avi":
		w.Header().Set("Content-Type", "video/x-msvideo")
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	}

	// CORS headers for Safari video playback
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Range, Content-Length, Content-Type")

	// Enable range requests for video streaming (Safari requirement)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filepath.Base(fullPath)))

	// For MP4 files, optimize for Safari streaming if needed
	if ext == ".mp4" && mode == "inline" {
		optimizedPath := fullPath + ".optimized.mp4"

		// Check if optimized version exists and is newer than original
		optimizedInfo, err := os.Stat(optimizedPath)
		needsOptimization := true

		if err == nil && optimizedInfo.ModTime().After(info.ModTime()) {
			// Use cached optimized version
			fullPath = optimizedPath
			needsOptimization = false
		}

		if needsOptimization {
			// Optimize MP4 with faststart for Safari
			cmd := exec.Command("ffmpeg", "-i", fullPath, "-c", "copy", "-movflags", "+faststart", "-y", optimizedPath)
			if err := cmd.Run(); err == nil {
				// Successfully optimized, use the new file
				fullPath = optimizedPath
			}
			// If optimization fails, serve original file
		}
	}

	http.ServeFile(w, r, fullPath)
}

// CreateFolder creates a new folder
func (h *FileManagerHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fullPath, err := h.resolvePath(req.Root, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	newFolderPath := filepath.Join(fullPath, req.Name)
	if err := os.MkdirAll(newFolderPath, 0777); err != nil {
		http.Error(w, "Failed to create folder: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// DeleteItem deletes a file or directory (Soft Delete to Trash)
func (h *FileManagerHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	rootKey := r.URL.Query().Get("root")
	subPath := r.URL.Query().Get("path")

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Prevent deleting root itself (simple check)
	baseRoot := h.RootPaths[rootKey]
	if fullPath == baseRoot {
		http.Error(w, "Cannot delete root directory", http.StatusForbidden)
		return
	}

	// If deleting FROM trash, delete permanently
	if rootKey == "trash" {
		if err := os.RemoveAll(fullPath); err != nil {
			http.Error(w, "Failed to delete item: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Soft Delete: Move to Trash
	trashRoot := h.RootPaths["trash"]
	if _, err := os.Stat(trashRoot); os.IsNotExist(err) {
		os.MkdirAll(trashRoot, 0777)
	}

	fileName := filepath.Base(fullPath)
	destPath := filepath.Join(trashRoot, fileName)

	// Avoid Name Collision
	if _, err := os.Stat(destPath); err == nil {
		timestamp := time.Now().Format("20060102150405")
		ext := filepath.Ext(fileName)
		name := strings.TrimSuffix(fileName, ext)
		destPath = filepath.Join(trashRoot, name+"_"+timestamp+ext)
	}

	// Move
	if err := os.Rename(fullPath, destPath); err != nil {
		// Handle cross-device
		if strings.Contains(strings.ToLower(err.Error()), "cross-device") || strings.Contains(strings.ToLower(err.Error()), "exdev") {
			cmd := exec.Command("cp", "-r", fullPath, destPath)
			if err := cmd.Run(); err != nil {
				http.Error(w, "Failed to move to trash: "+err.Error(), http.StatusInternalServerError)
				return
			}
			os.RemoveAll(fullPath)
		} else {
			http.Error(w, "Failed to move to trash: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// EmptyTrash permanently empties the trash (requires Password + 2FA)
func (h *FileManagerHandler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Authentication
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get User
	user, err := h.UserService.Repo.Get(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Verify User has 2FA enabled
	if !user.TOTPEnabled {
		http.Error(w, "2FA must be enabled to empty trash", http.StatusForbidden)
		return
	}

	// Verify Password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Verify 2FA
	ip := r.RemoteAddr
	valid, err := h.TOTPService.Verify(r.Context(), userID, req.Code, ip)
	if err != nil || !valid {
		http.Error(w, "Invalid 2FA code", http.StatusUnauthorized)
		return
	}

	// Empty Trash
	trashRoot := h.RootPaths["trash"]
	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		http.Error(w, "Failed to read trash: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		os.RemoveAll(filepath.Join(trashRoot, entry.Name()))
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// MoveItem moves a file/folder from one location to another (Cut/Paste)
func (h *FileManagerHandler) MoveItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceRoot string `json:"sourceRoot"`
		SourcePath string `json:"sourcePath"`
		DestRoot   string `json:"destRoot"`
		DestPath   string `json:"destPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	srcPath, err := h.resolvePath(req.SourceRoot, req.SourcePath)
	if err != nil {
		http.Error(w, "Invalid source path: "+err.Error(), http.StatusForbidden)
		return
	}

	destDir, err := h.resolvePath(req.DestRoot, req.DestPath)
	if err != nil {
		http.Error(w, "Invalid destination path: "+err.Error(), http.StatusForbidden)
		return
	}

	// Get source filename
	srcName := filepath.Base(srcPath)
	destPath := filepath.Join(destDir, srcName)

	// Attempt to move
	if err := os.Rename(srcPath, destPath); err != nil {
		// Check if it's a cross-device link error
		if strings.Contains(strings.ToLower(err.Error()), "cross-device") || strings.Contains(strings.ToLower(err.Error()), "invalid cross-device link") {
			// Use cp for cross-filesystem moves
			cmd := exec.Command("cp", "-r", srcPath, destPath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to move item: %v (output: %s)", err, out), http.StatusInternalServerError)
				return
			}
			os.RemoveAll(srcPath)
		} else {
			http.Error(w, "Failed to move item: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// getDirSize calculates directory size recursively
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// GenerateThumbnail generates a thumbnail for an image or video.
// For videos, it uses ffmpeg to extract the first frame and caches it.
// For images, it serves the original file (browsers handle scaling).
func (h *FileManagerHandler) GenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	rootKey := r.URL.Query().Get("root")
	subPath := r.URL.Query().Get("path")

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	videoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".m4v": true}

	if !videoExts[ext] {
		// For images, serve the file directly
		http.ServeFile(w, r, fullPath)
		return
	}

	// For videos: generate thumbnail using ffmpeg and cache it
	thumbDir := filepath.Join(filepath.Dir(fullPath), ".thumbs")
	baseName := strings.TrimSuffix(filepath.Base(fullPath), ext)
	thumbPath := filepath.Join(thumbDir, baseName+".jpg")

	// Check if cached thumbnail exists
	if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, thumbPath)
		return
	}

	// Create thumbs directory
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		log.Printf("Failed to create thumbs dir: %v", err)
		http.ServeFile(w, r, fullPath)
		return
	}

	// Generate thumbnail with ffmpeg
	cmd := exec.Command("ffmpeg",
		"-i", fullPath,
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-f", "image2",
		"-y",
		thumbPath,
	)
	if err := cmd.Run(); err != nil {
		log.Printf("ffmpeg thumbnail generation failed for %s: %v", fullPath, err)
		// Serve a 1x1 transparent pixel as fallback
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, thumbPath)
}

// StoreBackup saves a backup file to the backups root
// This exposes the file manager logic as an internal API
func (h *FileManagerHandler) StoreBackup(filename string, data []byte) (string, error) {
	fullPath, err := h.resolvePath("backups", filename)
	if err != nil {
		return "", err
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fullPath, nil
}

// GetBackup retrieves a backup file from the backups root
func (h *FileManagerHandler) GetBackup(filename string) ([]byte, error) {
	fullPath, err := h.resolvePath("backups", filename)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// ListBackups returns a list of backup files, implementing StorageProvider interface
func (h *FileManagerHandler) ListBackups() ([]services.StorageFileInfo, error) {
	rootPath, ok := h.RootPaths["backups"]
	if !ok {
		return nil, fmt.Errorf("backups root not configured")
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []services.StorageFileInfo{}, nil
		}
		return nil, err
	}

	var files []services.StorageFileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			continue
		}

		files = append(files, services.StorageFileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// Sort by mod time desc (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// DeleteBackup deletes a backup file from the backups root
func (h *FileManagerHandler) DeleteBackup(filename string) error {
	fullPath, err := h.resolvePath("backups", filename)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
