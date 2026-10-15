package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	buffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = buffer
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	type outputStruct struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	var output outputStruct

	err = json.Unmarshal(buffer.Bytes(), &output)
	if err != nil {
		return "", err
	}
	if output.Streams[0].Width/16 == output.Streams[0].Height/9 {
		return "16:9", nil
	}
	if output.Streams[0].Height/16 == output.Streams[0].Width/9 {
		return "9:16", nil
	}
	return "other", nil
}
