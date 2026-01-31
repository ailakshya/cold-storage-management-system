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
	R2Backend   *services.S3Backend  // Cloudflare R2 media bucket (nil if not configured)
	NASBackend  *services.S3Backend  // RustFS/MinIO on TrueNAS (nil if not configured)
}

func (h *FileManagerHandler) SetR2Backend(b *services.S3Backend)  { h.R2Backend = b }
func (h *FileManagerHandler) SetNASBackend(b *services.S3Backend) { h.NASBackend = b }

// getS3Backend returns the S3Backend for the given root key, or nil if not an S3 root.
func (h *FileManagerHandler) getS3Backend(rootKey string) *services.S3Backend {
	switch rootKey {
	case "r2":
		return h.R2Backend
	case "nas":
		return h.NASBackend
	default:
		return nil
	}
}

// isS3Root returns true if the root key maps to an S3 backend.
func (h *FileManagerHandler) isS3Root(rootKey string) bool {
	return rootKey == "r2" || rootKey == "nas"
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

	// Add S3 cloud storage entries (R2 / NAS)
	if h.R2Backend != nil {
		stats = append(stats, StorageStats{
			Root:  "r2",
			Label: "Cloudflare R2",
			Path:  "cloud://r2",
		})
	}
	if h.NASBackend != nil {
		stats = append(stats, StorageStats{
			Root:  "nas",
			Label: "NAS (RustFS)",
			Path:  "s3://nas",
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

	// S3 backend dispatch (R2 / NAS)
	if backend := h.getS3Backend(rootKey); backend != nil {
		h.listS3Files(w, r, backend, subPath, searchQuery)
		return
	}

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

	// S3 backend dispatch (R2 / NAS)
	if backend := h.getS3Backend(rootKey); backend != nil {
		h.uploadS3File(w, r, backend, subPath)
		return
	}

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

	// Set permissions
	os.Chmod(destPath, 0666)

	// Synchronous video conversion - convert before returning to ensure file is ready
	finalPath := destPath
	ext := strings.ToLower(filepath.Ext(destPath))
	videoExts := map[string]bool{
		".mp4": true, ".mov": true, ".m4v": true, ".mkv": true,
		".avi": true, ".webm": true, ".flv": true, ".wmv": true,
		".3gp": true, ".mts": true,
	}

	if videoExts[ext] {
		// Convert video in background - don't block upload response
		convertVideoInBackground(destPath)
		log.Printf("[Upload] Video conversion queued for background processing: %s", filepath.Base(destPath))
		// Always predict lowercase .mp4 - the converter always outputs lowercase
		// This handles .MP4/.MOV etc. from phones where ext is lowered but destPath keeps original case
		finalPath = strings.TrimSuffix(destPath, filepath.Ext(destPath)) + ".mp4"
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":       "success",
		"file":         header.Filename,
		"converted_to": filepath.Base(finalPath),
	})
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
	dst.Close()

	// Race-safe chunk assembly: count only chunk files, use lock file to prevent multiple assemblers
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "chunk_uploaded"})
		return
	}

	// Count chunk files (exclude lock file and other metadata)
	chunkCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk_") {
			chunkCount++
		}
	}

	if chunkCount >= totalChunks {
		// All chunks received - try to acquire assembly lock
		lockPath := filepath.Join(tempDir, ".assembling")
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			// Another request is already assembling - just return success
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "chunk_uploaded"})
			return
		}
		lockFile.Close()

		// We have the lock - assemble the file
		// Ensure destination directory exists
		if err := os.MkdirAll(fullPath, 0777); err != nil {
			os.RemoveAll(tempDir)
			http.Error(w, "Failed to create destination directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		finalDestPath := filepath.Join(fullPath, filepath.Base(filename))

		// Create final file
		finalFile, err := os.Create(finalDestPath)
		if err != nil {
			os.RemoveAll(tempDir)
			http.Error(w, "Failed to create final file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Append chunks in order
		assemblyFailed := false
		for i := 0; i < totalChunks; i++ {
			chunkPartPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d", i))
			chunkData, err := os.ReadFile(chunkPartPath)
			if err != nil {
				log.Printf("[UploadChunk] Failed to read chunk %d: %v", i, err)
				assemblyFailed = true
				break
			}
			if _, err := finalFile.Write(chunkData); err != nil {
				log.Printf("[UploadChunk] Failed to write chunk %d: %v", i, err)
				assemblyFailed = true
				break
			}
		}
		finalFile.Close()

		if assemblyFailed {
			os.Remove(finalDestPath)
			os.RemoveAll(tempDir)
			http.Error(w, "Failed to assemble chunks", http.StatusInternalServerError)
			return
		}

		// Cleanup temp dir
		os.RemoveAll(tempDir)

		// Set permissions
		os.Chmod(finalDestPath, 0666)

		// Synchronous video conversion - convert before returning to ensure file is ready
		finalPath := finalDestPath
		ext := strings.ToLower(filepath.Ext(finalDestPath))
		videoExts := map[string]bool{
			".mp4": true, ".mov": true, ".m4v": true, ".mkv": true,
			".avi": true, ".webm": true, ".flv": true, ".wmv": true,
			".3gp": true, ".mts": true,
		}

		if videoExts[ext] {
			// Convert video in background - don't block upload response
			convertVideoInBackground(finalDestPath)
			log.Printf("[UploadChunk] Video conversion queued for background processing: %s", filepath.Base(finalDestPath))
			// Always predict lowercase .mp4 - the converter always outputs lowercase
			finalPath = strings.TrimSuffix(finalDestPath, filepath.Ext(finalDestPath)) + ".mp4"
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":       "completed",
			"file":         filepath.Base(filename),
			"converted_to": filepath.Base(finalPath),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "chunk_uploaded"})
}

// convertVideoToH264 converts any video to browser-compatible H.264 MP4.
// Instagram/Facebook-style processing: always transcode for consistency.
// Features:
// - Always transcodes (no remux) for guaranteed H.264 output
// - Caps resolution at 1080p (max width 1920, height scales proportionally)
// - Uses CRF 24 for good quality/size balance
// - Adds faststart for web streaming
// - Returns the new MP4 path
func convertVideoToH264(inputPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	originalExt := filepath.Ext(inputPath) // Preserve original case for TrimSuffix (case-sensitive)

	// Supported input formats (including MP4 which may have HEVC codec)
	videoExts := map[string]bool{
		".mp4": true, ".mov": true, ".m4v": true, ".mkv": true,
		".avi": true, ".webm": true, ".flv": true, ".wmv": true,
		".3gp": true, ".mts": true,
	}
	if !videoExts[ext] {
		return inputPath, nil // Not a video, return as-is
	}

	// Output path (always .mp4) — use originalExt for TrimSuffix since it's case-sensitive
	mp4Path := strings.TrimSuffix(inputPath, originalExt) + ".mp4"

	// If input is already .mp4, use temp output to avoid overwriting during conversion
	tempPath := mp4Path
	needsRename := false
	if ext == ".mp4" {
		tempPath = strings.TrimSuffix(inputPath, originalExt) + "_h264_temp.mp4"
		needsRename = true
	}

	log.Printf("[VideoConvert] Converting %s to H.264 MP4 (max 1080p, CRF 24)...", filepath.Base(inputPath))

	// FFmpeg command with Instagram-like settings:
	// - Scale to max 1080p width, maintaining aspect ratio (-2 ensures height is divisible by 2)
	// - H.264 video codec (libx264) for universal browser support
	// - AAC audio at 128k
	// - CRF 23 for good quality (Instagram uses 23-26)
	// - Fast preset for quick encoding (2-3x faster than medium, slightly larger files)
	// - faststart for progressive web playback
	// - yuv420p pixel format for Safari/browser compatibility
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:a", "128k",
		"-preset", "fast",
		"-crf", "23",
		"-vf", "scale='min(1920,iw)':-2", // Max width 1920, height auto (divisible by 2)
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p", // Required for Safari/older browsers
		"-threads", "0", // Use all CPU cores
		"-y", // Overwrite output
		tempPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[VideoConvert] FFmpeg failed for %s: %v\nOutput: %s",
			filepath.Base(inputPath), err, string(output))
		return inputPath, fmt.Errorf("conversion failed: %w", err)
	}

	// If we used a temp path (for MP4 input), rename to final path
	if needsRename {
		// Remove original first
		os.Remove(inputPath)
		if err := os.Rename(tempPath, mp4Path); err != nil {
			// If rename fails, keep temp file with different name
			log.Printf("[VideoConvert] Warning: rename failed, keeping temp file: %v", err)
			return tempPath, nil
		}
	} else {
		// Remove original (if different from output)
		if inputPath != mp4Path {
			if err := os.Remove(inputPath); err != nil {
				log.Printf("[VideoConvert] Warning: failed to remove original %s: %v",
					filepath.Base(inputPath), err)
			}
		}
	}

	// Get file sizes for logging
	inputInfo, _ := os.Stat(inputPath)
	outputInfo, _ := os.Stat(mp4Path)
	inputSize := int64(0)
	outputSize := int64(0)
	if inputInfo != nil {
		inputSize = inputInfo.Size()
	}
	if outputInfo != nil {
		outputSize = outputInfo.Size()
	}

	log.Printf("[VideoConvert] Success: %s → %s (%.1fMB → %.1fMB)",
		filepath.Base(inputPath), filepath.Base(mp4Path),
		float64(inputSize)/1024/1024, float64(outputSize)/1024/1024)

	return mp4Path, nil
}

// isVideoExt returns true if the lowercased extension is a known video format.
func isVideoExt(lowExt string) bool {
	switch lowExt {
	case ".mp4", ".mov", ".m4v", ".mkv", ".avi", ".webm", ".flv", ".wmv", ".3gp", ".mts":
		return true
	}
	return false
}

// convertVideoInBackground is kept for backward compatibility but now uses the new H.264 conversion.
// Runs conversion in background goroutine.
func convertVideoInBackground(path string) {
	go func() {
		_, err := convertVideoToH264(path)
		if err != nil {
			log.Printf("[VideoConvert] Background conversion failed: %v", err)
		}
	}()
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

	// S3 backend dispatch (R2 / NAS)
	if backend := h.getS3Backend(rootKey); backend != nil {
		h.downloadS3File(w, r, backend, subPath, mode)
		return
	}

	fullPath, err := h.resolvePath(rootKey, subPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		// For video files, check if a converted MP4 version exists
		// Background conversion may have replaced the original (e.g., .MOV → .mp4)
		lowExt := strings.ToLower(filepath.Ext(fullPath))
		if lowExt != ".mp4" && isVideoExt(lowExt) {
			origExt := filepath.Ext(fullPath)
			// Try correct conversion name: file.mp4
			mp4Try := strings.TrimSuffix(fullPath, origExt) + ".mp4"
			if fi, e := os.Stat(mp4Try); e == nil {
				fullPath = mp4Try
				info = fi
			} else {
				// Try legacy buggy conversion name: file.MOV.mp4
				mp4Try2 := fullPath + ".mp4"
				if fi, e := os.Stat(mp4Try2); e == nil {
					fullPath = mp4Try2
					info = fi
				}
			}
		}
		if info == nil {
			// Fallback: try downloading from cloud backends (NAS → R2)
			if h.NASBackend != nil || h.R2Backend != nil {
				if h.tryCloudFallback(w, r, subPath, mode) {
					return
				}
			}
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
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

	// CORS is handled by Caddy (direct connection) or same-origin (Cloudflare tunnel).
	// Do NOT set CORS headers here — duplicate headers conflict with Caddy's and cause browsers to block responses.

	// Enable range requests for video streaming (Safari requirement)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filepath.Base(fullPath)))

	// For MOV files, convert to MP4 on-demand for browser playback (HEVC not supported)
	if ext == ".mov" && mode == "inline" {
		actualExt := filepath.Ext(fullPath) // Preserve original case for TrimSuffix
		mp4Path := strings.TrimSuffix(fullPath, actualExt) + ".mp4"

		// Check if MP4 version already exists
		if mp4Info, err := os.Stat(mp4Path); err == nil && mp4Info.Size() > 0 {
			// Use existing MP4 version
			fullPath = mp4Path
			w.Header().Set("Content-Type", "video/mp4")
		} else {
			// Convert MOV to MP4 (try fast remux first, fallback to re-encode)
			log.Printf("[VideoConvert] On-demand converting %s to MP4", filepath.Base(fullPath))
			cmd := exec.Command("ffmpeg", "-i", fullPath, "-c", "copy", "-movflags", "+faststart", "-y", mp4Path)
			if err := cmd.Run(); err != nil {
				// Fallback to re-encode for HEVC content
				cmd = exec.Command("ffmpeg", "-i", fullPath, "-c:v", "libx264", "-c:a", "aac",
					"-preset", "fast", "-crf", "23", "-movflags", "+faststart", "-threads", "0", "-y", mp4Path)
				if err := cmd.Run(); err == nil {
					fullPath = mp4Path
					w.Header().Set("Content-Type", "video/mp4")
				}
				// If conversion fails, serve original MOV
			} else {
				fullPath = mp4Path
				w.Header().Set("Content-Type", "video/mp4")
			}
		}
	}

	// Note: MP4 faststart optimization is applied during upload-time conversion.
	// No need for on-demand optimization here — it would block the response for large files.

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

	// S3 backend dispatch (R2 / NAS) — permanent delete, no trash
	if backend := h.getS3Backend(rootKey); backend != nil {
		h.deleteS3Item(w, r, backend, subPath)
		return
	}

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

	// S3 backend dispatch — handle moves involving R2 or NAS
	if h.isS3Root(req.SourceRoot) || h.isS3Root(req.DestRoot) {
		h.moveItemCrossStorage(w, r, req.SourceRoot, req.SourcePath, req.DestRoot, req.DestPath)
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

// ---------------------------------------------------------------------------
// S3 backend operations (Cloudflare R2, RustFS/MinIO on TrueNAS)
// ---------------------------------------------------------------------------

// listS3Files lists objects in an S3 bucket using prefix-based navigation.
func (h *FileManagerHandler) listS3Files(w http.ResponseWriter, r *http.Request, backend *services.S3Backend, subPath, searchQuery string) {
	ctx := r.Context()

	objects, err := backend.List(ctx, subPath)
	if err != nil {
		http.Error(w, "Failed to list files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	files := []FileInfo{}
	for _, obj := range objects {
		fileType := getFileTypeFromName(obj.IsDir, obj.Name)

		// If searching, filter by name
		if searchQuery != "" && !strings.Contains(strings.ToLower(obj.Name), searchQuery) {
			continue
		}

		files = append(files, FileInfo{
			Name:    obj.Name,
			Path:    obj.Key,
			IsDir:   obj.IsDir,
			Size:    obj.Size,
			ModTime: obj.ModTime,
			Type:    fileType,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListResponse{
		Files:       files,
		CurrentPath: subPath,
	})
}

// downloadS3File streams an object from S3 to the HTTP response.
func (h *FileManagerHandler) downloadS3File(w http.ResponseWriter, r *http.Request, backend *services.S3Backend, key, mode string) {
	reader, size, err := backend.Download(r.Context(), key)
	if err != nil {
		http.Error(w, "File not found: "+err.Error(), http.StatusNotFound)
		return
	}
	defer reader.Close()

	fileName := filepath.Base(key)
	disposition := "attachment"
	if mode == "inline" {
		disposition = "inline"
	}

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".mov":
		w.Header().Set("Content-Type", "video/quicktime")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".pdf":
		w.Header().Set("Content-Type", "application/pdf")
	case ".sql":
		w.Header().Set("Content-Type", "application/sql")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName))
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	io.Copy(w, reader)
}

// uploadS3File handles uploading a file to an S3 backend.
func (h *FileManagerHandler) uploadS3File(w http.ResponseWriter, r *http.Request, backend *services.S3Backend, subPath string) {
	ctx := r.Context()

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Build S3 key: subpath/filename
	key := header.Filename
	if subPath != "" {
		key = strings.TrimSuffix(subPath, "/") + "/" + header.Filename
	}

	if err := backend.Upload(ctx, key, file, header.Size); err != nil {
		http.Error(w, "Upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"file":   header.Filename,
	})
}

// deleteS3Item deletes an object or prefix from an S3 backend.
func (h *FileManagerHandler) deleteS3Item(w http.ResponseWriter, r *http.Request, backend *services.S3Backend, key string) {
	ctx := r.Context()

	if err := backend.Delete(ctx, key); err != nil {
		http.Error(w, "Delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// moveItemCrossStorage handles moves where at least one side is an S3 backend.
// Supports: S3↔S3 (same or different), Local→S3, S3→Local.
func (h *FileManagerHandler) moveItemCrossStorage(w http.ResponseWriter, r *http.Request,
	srcRoot, srcPath, dstRoot, dstPath string) {
	ctx := r.Context()

	srcBackend := h.getS3Backend(srcRoot)
	dstBackend := h.getS3Backend(dstRoot)

	// Extract source filename
	srcFileName := filepath.Base(srcPath)

	// Build destination key/path: destPath/srcFileName
	dstKey := srcFileName
	if dstPath != "" {
		dstKey = strings.TrimSuffix(dstPath, "/") + "/" + srcFileName
	}

	// Case 1: S3 → S3 (same or different backends)
	if srcBackend != nil && dstBackend != nil {
		var err error
		if srcBackend == dstBackend {
			err = srcBackend.Move(ctx, srcPath, dstKey)
		} else {
			err = services.CrossBackendMove(ctx, srcBackend, srcPath, dstBackend, dstKey)
		}
		if err != nil {
			http.Error(w, "Move failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Case 2: Local → S3
	if srcBackend == nil && dstBackend != nil {
		localPath, err := h.resolvePath(srcRoot, srcPath)
		if err != nil {
			http.Error(w, "Invalid source path: "+err.Error(), http.StatusForbidden)
			return
		}

		file, err := os.Open(localPath)
		if err != nil {
			http.Error(w, "Failed to open source file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to stat source file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := dstBackend.Upload(ctx, dstKey, file, info.Size()); err != nil {
			http.Error(w, "Upload failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Delete local source after successful upload
		os.RemoveAll(localPath)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Case 3: S3 → Local
	if srcBackend != nil && dstBackend == nil {
		reader, _, err := srcBackend.Download(ctx, srcPath)
		if err != nil {
			http.Error(w, "Download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer reader.Close()

		localDir, err := h.resolvePath(dstRoot, dstPath)
		if err != nil {
			http.Error(w, "Invalid destination path: "+err.Error(), http.StatusForbidden)
			return
		}

		if err := os.MkdirAll(localDir, 0777); err != nil {
			http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		localFile := filepath.Join(localDir, srcFileName)
		f, err := os.Create(localFile)
		if err != nil {
			http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(f, reader); err != nil {
			http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Delete S3 source after successful download
		if err := srcBackend.Delete(ctx, srcPath); err != nil {
			log.Printf("[FileManager] Warning: moved to local but failed to delete S3 source: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}
}

// tryCloudFallback attempts to serve a file from NAS or R2 when the local copy is missing.
// Tries multiple S3 key patterns (subPath as-is, stripped "media/" prefix).
// Returns true if the file was served from a cloud backend.
func (h *FileManagerHandler) tryCloudFallback(w http.ResponseWriter, r *http.Request, subPath, mode string) bool {
	// Build candidate S3 keys to try
	candidates := []string{subPath}
	if strings.HasPrefix(subPath, "media/") {
		candidates = append(candidates, strings.TrimPrefix(subPath, "media/"))
	}

	// Try backends in order: NAS first (faster, local network), then R2
	backends := []*services.S3Backend{}
	if h.NASBackend != nil {
		backends = append(backends, h.NASBackend)
	}
	if h.R2Backend != nil {
		backends = append(backends, h.R2Backend)
	}

	for _, key := range candidates {
		for _, backend := range backends {
			reader, size, err := backend.Download(r.Context(), key)
			if err != nil {
				continue
			}

			log.Printf("[FileManager] Cloud fallback: serving %s from %s", key, backend.Name())

			fileName := filepath.Base(key)
			disposition := "attachment"
			if mode == "inline" {
				disposition = "inline"
			}

			ext := strings.ToLower(filepath.Ext(fileName))
			switch ext {
			case ".mp4":
				w.Header().Set("Content-Type", "video/mp4")
			case ".mov":
				w.Header().Set("Content-Type", "video/quicktime")
			case ".webm":
				w.Header().Set("Content-Type", "video/webm")
			case ".jpg", ".jpeg":
				w.Header().Set("Content-Type", "image/jpeg")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			case ".gif":
				w.Header().Set("Content-Type", "image/gif")
			case ".webp":
				w.Header().Set("Content-Type", "image/webp")
			case ".pdf":
				w.Header().Set("Content-Type", "application/pdf")
			}

			w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName))
			if size > 0 {
				w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			}

			io.Copy(w, reader)
			reader.Close()
			return true
		}
	}
	return false
}

// getFileTypeFromName returns the file type string based on name and extension.
func getFileTypeFromName(isDir bool, name string) string {
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
