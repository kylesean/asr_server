package speaker

import (
	"asr_server/config"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-audio/wav"
)

// Handler handles speaker recognition HTTP requests.
// All dependencies are explicitly injected via constructor.
type Handler struct {
	manager *Manager
	cfg     *config.Config
}

// NewHandler creates a new handler with explicit dependencies
func NewHandler(manager *Manager, cfg *config.Config) *Handler {
	return &Handler{
		manager: manager,
		cfg:     cfg,
	}
}

// RegisterRoutes registers routes
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	speakerGroup := router.Group("/api/v1/speaker")
	{
		speakerGroup.POST("/register", h.RegisterSpeaker)
		speakerGroup.POST("/identify", h.IdentifySpeaker)
		speakerGroup.POST("/verify/:speaker_id", h.VerifySpeaker)
		speakerGroup.GET("/list", h.GetAllSpeakers)
		speakerGroup.DELETE("/:speaker_id", h.DeleteSpeaker)
		speakerGroup.GET("/stats", h.GetStats)
		speakerGroup.POST("/register_base64", h.RegisterSpeakerBase64)
		speakerGroup.POST("/identify_base64", h.IdentifySpeakerBase64)
	}
}

// RegisterSpeaker registers a speaker
func (h *Handler) RegisterSpeaker(c *gin.Context) {
	speakerID := c.PostForm("speaker_id")
	speakerName := c.PostForm("speaker_name")

	if speakerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "speaker_id is required",
		})
		return
	}

	if speakerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "speaker_name is required",
		})
		return
	}

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "audio file is required",
		})
		return
	}
	defer file.Close()

	audioData, sampleRate, err := h.parseAudioFile(file, header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to parse audio file: %v", err),
		})
		return
	}

	err = h.manager.RegisterSpeaker(speakerID, speakerName, audioData, sampleRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to register speaker: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Speaker registered successfully",
		"speaker_id":   speakerID,
		"speaker_name": speakerName,
	})
}

// IdentifySpeaker identifies a speaker
func (h *Handler) IdentifySpeaker(c *gin.Context) {
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "audio file is required",
		})
		return
	}
	defer file.Close()

	audioData, sampleRate, err := h.parseAudioFile(file, header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to parse audio file: %v", err),
		})
		return
	}

	result, err := h.manager.IdentifySpeaker(audioData, sampleRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to identify speaker: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// VerifySpeaker verifies a speaker
func (h *Handler) VerifySpeaker(c *gin.Context) {
	speakerID := c.Param("speaker_id")
	if speakerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "speaker_id is required",
		})
		return
	}

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "audio file is required",
		})
		return
	}
	defer file.Close()

	audioData, sampleRate, err := h.parseAudioFile(file, header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to parse audio file: %v", err),
		})
		return
	}

	result, err := h.manager.VerifySpeaker(speakerID, audioData, sampleRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to verify speaker: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetAllSpeakers returns all speakers
func (h *Handler) GetAllSpeakers(c *gin.Context) {
	speakers := h.manager.GetAllSpeakers()
	c.JSON(http.StatusOK, gin.H{
		"speakers": speakers,
		"total":    len(speakers),
	})
}

// DeleteSpeaker deletes a speaker
func (h *Handler) DeleteSpeaker(c *gin.Context) {
	speakerID := c.Param("speaker_id")
	if speakerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "speaker_id is required",
		})
		return
	}

	err := h.manager.DeleteSpeaker(speakerID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to delete speaker: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Speaker deleted successfully",
		"speaker_id": speakerID,
	})
}

// GetStats returns database statistics
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetDatabaseStats()
	c.JSON(http.StatusOK, stats)
}

// parseAudioFile parses an audio file
func (h *Handler) parseAudioFile(file multipart.File, header *multipart.FileHeader) ([]float32, int, error) {
	filename := strings.ToLower(header.Filename)
	if !strings.HasSuffix(filename, ".wav") {
		return nil, 0, fmt.Errorf("only WAV files are supported")
	}

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}

	sampleRate := int(decoder.SampleRate)
	numChannels := int(decoder.NumChans)

	if numChannels > 2 {
		return nil, 0, fmt.Errorf("unsupported number of channels: %d", numChannels)
	}

	buffer, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode audio: %v", err)
	}

	// Convert to float32 format using config
	samples := make([]float32, len(buffer.Data))
	normalizeFactor := h.cfg.Audio.NormalizeFactor
	for i, sample := range buffer.Data {
		samples[i] = float32(sample) / normalizeFactor
	}

	// Convert stereo to mono if needed
	if numChannels == 2 {
		monoSamples := make([]float32, len(samples)/2)
		for i := 0; i < len(monoSamples); i++ {
			monoSamples[i] = (samples[i*2] + samples[i*2+1]) / 2.0
		}
		samples = monoSamples
	}

	return samples, sampleRate, nil
}

// RegisterSpeakerBase64 registers a speaker using Base64 encoded audio
func (h *Handler) RegisterSpeakerBase64(c *gin.Context) {
	var req struct {
		SpeakerID   string `json:"speaker_id" binding:"required"`
		SpeakerName string `json:"speaker_name" binding:"required"`
		AudioData   string `json:"audio_data" binding:"required"`
		SampleRate  int    `json:"sample_rate" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Base64 API not implemented yet",
	})
}

// IdentifySpeakerBase64 identifies a speaker using Base64 encoded audio
func (h *Handler) IdentifySpeakerBase64(c *gin.Context) {
	var req struct {
		AudioData  string `json:"audio_data" binding:"required"`
		SampleRate int    `json:"sample_rate" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Base64 API not implemented yet",
	})
}
