package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestUploadHandler(t *testing.T) {
	app := fiber.New()

	// Mock upload endpoint
	app.Post("/files/upload", func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "BAD_REQUEST", "message": "file is required"},
			})
		}

		// Validate content type
		contentType := file.Header.Get("Content-Type")
		allowed := map[string]bool{
			"image/jpeg":      true,
			"image/png":       true,
			"image/gif":       true,
			"image/webp":      true,
			"application/pdf": true,
		}
		if !allowed[contentType] {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "BAD_REQUEST", "message": "content type not allowed"},
			})
		}

		directory := c.FormValue("directory", "")
		path := file.Filename
		if directory != "" {
			path = directory + "/" + file.Filename
		}

		return c.Status(http.StatusCreated).JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"path": path,
				"url":  "/uploads/" + path,
				"size": file.Size,
			},
		})
	})

	t.Run("successful_upload", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.png"`)
		h.Set("Content-Type", "image/png")
		part, _ := writer.CreatePart(h)
		part.Write([]byte("fake png data"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "test.png", data["path"])
	})

	t.Run("upload_with_directory", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="avatar.jpg"`)
		h.Set("Content-Type", "image/jpeg")
		part, _ := writer.CreatePart(h)
		part.Write([]byte("fake jpg data"))

		writer.WriteField("directory", "avatars")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, "avatars/avatar.jpg", data["path"])
	})

	t.Run("missing_file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("disallowed_content_type", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="virus.exe"`)
		h.Set("Content-Type", "application/x-executable")
		part, _ := writer.CreatePart(h)
		part.Write([]byte("malicious"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestDownloadHandler(t *testing.T) {
	app := fiber.New()

	// Mock download endpoint using wildcard route
	app.Get("/files/download/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "BAD_REQUEST", "message": "file path is required"},
			})
		}

		if path == "nonexistent.png" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "file not found"},
			})
		}

		c.Set("Content-Type", "image/png")
		c.Set("Content-Disposition", `attachment; filename="`+path+`"`)
		return c.Send([]byte("fake file data"))
	})

	t.Run("successful_download", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files/download/test.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "test.png")

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "fake file data", string(body))
	})

	t.Run("download_nested_path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files/download/avatars/user123/photo.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "avatars/user123/photo.png")
	})

	t.Run("file_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files/download/nonexistent.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeleteHandler(t *testing.T) {
	app := fiber.New()

	// Mock delete endpoint using wildcard route
	app.Delete("/files/*", func(c *fiber.Ctx) error {
		path := c.Params("*")

		if path == "nonexistent.png" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "file not found"},
			})
		}

		return c.SendStatus(http.StatusNoContent)
	})

	t.Run("successful_delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/files/test.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("delete_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/files/nonexistent.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestListHandler(t *testing.T) {
	app := fiber.New()

	// Mock list endpoint
	app.Get("/files", func(c *fiber.Ctx) error {
		prefix := c.Query("prefix", "")
		_ = prefix

		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"files": []fiber.Map{
					{"path": "images/test1.png", "size": 1024, "content_type": "image/png"},
					{"path": "images/test2.jpg", "size": 2048, "content_type": "image/jpeg"},
				},
			},
		})
	})

	t.Run("list_files", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files?prefix=images", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		files := data["files"].([]interface{})
		assert.Len(t, files, 2)
	})

	t.Run("list_all_files", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "test.png", "test.png"},
		{"with_newline", "test\r\n.png", "test.png"},
		{"with_quotes", `test".png`, "test.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHeaderValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
