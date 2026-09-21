package model

import "time"

type ImageInfo struct {
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Image     string     `json:"image"`
	Group     string     `json:"group"`
	Present   bool       `json:"present"`
	Size      int64      `json:"size,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

type ResponseImageList struct {
	Images []ImageInfo `json:"images"`
}

type ResponseImageAction struct {
	Message string `json:"message"`
}
