package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", nil)
		return
	}

	const maxMemory = 1 << 30
	http.MaxBytesReader(w, r.Body, maxMemory)
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("video")
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
	if !(mediaType == "video/mp4") {
		respondWithError(w, http.StatusBadRequest, "video must be a video of type mp4", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, 500, "could not create temp file", err)
		return
	}

	defer os.Remove("tubely-upload.mp4")
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not copy video data", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, 500, "File manipulation unsucsessful", err)
		return
	}

	rng := make([]byte, 32)
	rand.Read(rng)
	rngString := base64.RawURLEncoding.EncodeToString(rng)

	ratio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get aspect ratio", err)
		return
	}
	prefix := "other"
	if ratio == "16:9" {
		prefix = "landscape"
	}
	if ratio == "9:16" {
		prefix = "portrait"
	}

	key := fmt.Sprintf("%v/%v.mp4", prefix, rngString)

	putObjInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		ContentType: &contentType,
		Body:        tempFile,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &putObjInput)
	if err != nil {
		respondWithError(w, 500, "Couldn't upload file to aws", nil)
		return
	}

	videoURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, key)
	video.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, 500, "Couldn't update video", err)
		return
	}
	w.WriteHeader(200)
}
