package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse Content-Type", err)
		return
	}
	if !(mediaType == "image/jpeg" || mediaType == "image/png") {
		respondWithError(w, http.StatusBadRequest, "thumbnail must be an image of type jpeg or png", err)
		return
	}

	typeParts := strings.Split(mediaType, "/")
	extension := typeParts[1]

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	thumbnailPath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%v", videoID, extension))
	osFile, err := os.Create(thumbnailPath)
	if err != nil {
		respondWithError(w, 500, "could not create file", err)
		return
	}

	_, err = io.Copy(osFile, file)
	if err != nil {
		respondWithError(w, 500, "could not create file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v.%v", cfg.port, videoID, extension)
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, 500, "could not update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
