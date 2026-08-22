package email

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

func ExtractAttachments(data []byte) ([]Attachment, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return extractAttachmentsFromMsg(msg.Header, msg.Body)
}

func extractAttachmentsFromMsg(header mail.Header, body io.Reader) ([]Attachment, error) {
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))

	filename := ""
	cdHeader := header.Get("Content-Disposition")
	if cdHeader != "" {
		if cdType, cdParams, err := mime.ParseMediaType(cdHeader); err == nil {
			if cdType == "attachment" || cdType == "inline" {
				filename = cdParams["filename"]
			}
		}
	}
	if filename == "" && err == nil {
		if name, ok := params["name"]; ok {
			filename = name
		}
	}

	if filename != "" {
		dec := new(mime.WordDecoder)
		if decoded, err := dec.DecodeHeader(filename); err == nil {
			filename = decoded
		}
	}

	if filename != "" {
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		cte := strings.ToLower(header.Get("Content-Transfer-Encoding"))
		if cte == "base64" {
			decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
			if decodedData, err := io.ReadAll(decoder); err == nil {
				data = decodedData
			} else {
				lenient := base64.NewDecoder(base64.RawStdEncoding, bytes.NewReader(bytes.ReplaceAll(data, []byte("\r\n"), []byte(""))))
				if lData, err := io.ReadAll(lenient); err == nil {
					data = lData
				}
			}
		} else if cte == "quoted-printable" {
			decoder := quotedprintable.NewReader(bytes.NewReader(data))
			if decodedData, err := io.ReadAll(decoder); err == nil {
				data = decodedData
			}
		}
		return []Attachment{{
			Filename:    filename,
			ContentType: mediaType,
			Data:        data,
		}}, nil
	}

	var attachments []Attachment
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary != "" {
			mr := multipart.NewReader(body, boundary)
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				partHeader := mail.Header(part.Header)
				partAtts, err := extractAttachmentsFromMsg(partHeader, part)
				if err == nil {
					attachments = append(attachments, partAtts...)
				}
			}
		}
	}

	return attachments, nil
}

func HasICSAttachmentInMsg(header mail.Header, body io.Reader) bool {
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		ct := strings.ToLower(header.Get("Content-Type"))
		if strings.Contains(ct, "text/calendar") || strings.Contains(ct, "application/ics") || strings.Contains(ct, ".ics") {
			return true
		}
		return false
	}

	if name, ok := params["name"]; ok && strings.HasSuffix(strings.ToLower(name), ".ics") {
		return true
	}

	cdHeader := header.Get("Content-Disposition")
	if cdHeader != "" {
		cdType, cdParams, err := mime.ParseMediaType(cdHeader)
		if err == nil {
			if cdType == "attachment" {
				if filename, ok := cdParams["filename"]; ok && strings.HasSuffix(strings.ToLower(filename), ".ics") {
					return true
				}
			}
		}
	}

	if mediaType == "text/calendar" || mediaType == "application/ics" || mediaType == "text/x-vcalendar" {
		return true
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return false
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			partHeader := mail.Header(part.Header)
			if HasICSAttachmentInMsg(partHeader, part) {
				return true
			}
		}
	}

	return false
}

func HasAttachmentInMsg(header mail.Header, body io.Reader) bool {
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))

	cdHeader := header.Get("Content-Disposition")
	if cdHeader != "" {
		if cdType, _, errCD := mime.ParseMediaType(cdHeader); errCD == nil {
			if cdType == "attachment" {
				return true
			}
		}
	}
	if err == nil {
		if _, ok := params["name"]; ok {
			return true
		}
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return false
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, errPart := mr.NextPart()
			if errPart == io.EOF || errPart != nil {
				break
			}
			partHeader := mail.Header(part.Header)
			if HasAttachmentInMsg(partHeader, part) {
				return true
			}
		}
	}

	return false
}
