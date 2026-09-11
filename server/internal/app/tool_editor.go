package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Yanyutin753/loadout/server/internal/gateway"
)

func (a *application) previewSettlement(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Settlement json.RawMessage `json:"settlement"`
		Text       string          `json:"text"`
		IsError    bool            `json:"is_error"`
	}
	if !decodeLimit(w, r, &in, 512<<10) {
		return
	}
	if len(in.Text) > 64<<10 {
		fail(w, 400, "invalid_request")
		return
	}
	if !validSettlement(in.Settlement) {
		fail(w, 400, "invalid_settlement")
		return
	}
	// A fresh evaluator avoids caching unsaved preview scripts for the process lifetime.
	evaluator := new(gateway.Gateway)
	respond(w, 200, map[string]bool{"charge": evaluator.PreviewSettlement(in.Settlement, in.Text, in.IsError)})
}

func validToolIcon(icon string) bool {
	if icon == "" {
		return true
	}
	if strings.HasPrefix(icon, "https://") {
		u, err := url.Parse(icon)
		return err == nil && u.Hostname() != "" && u.User == nil && len(icon) <= 2048
	}
	header, data, ok := strings.Cut(icon, ",")
	if !ok || len(data) > base64.StdEncoding.EncodedLen(64<<10) {
		return false
	}
	switch header {
	case "data:image/svg+xml;base64", "data:image/png;base64", "data:image/jpeg;base64", "data:image/webp;base64":
	default:
		return false
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(data)
	if err != nil || len(decoded) == 0 || len(decoded) > 64<<10 {
		return false
	}
	switch header {
	case "data:image/png;base64":
		return bytes.HasPrefix(decoded, []byte("\x89PNG\r\n\x1a\n"))
	case "data:image/jpeg;base64":
		return bytes.HasPrefix(decoded, []byte{0xff, 0xd8, 0xff})
	case "data:image/webp;base64":
		return len(decoded) >= 12 && string(decoded[:4]) == "RIFF" && string(decoded[8:12]) == "WEBP"
	}
	return safeSVG(decoded)
}

var svgDeclaration = regexp.MustCompile(`^version\s*=\s*(?:"1\.0"|'1\.0')(?:\s+encoding\s*=\s*(?:"(?i:utf-8)"|'(?i:utf-8)'))?(?:\s+standalone\s*=\s*(?:"(?:yes|no)"|'(?:yes|no)'))?\s*$`)

// SVG is a static drawing vocabulary. No CSS, links, animation, foreign content,
// entities or processing instructions can introduce active/external resources.
// Only a single initial XML declaration is permitted.
func safeSVG(raw []byte) bool {
	elements := " svg g defs text tspan path rect circle ellipse line polyline polygon title desc linearGradient radialGradient stop clipPath mask "
	attributes := " id viewBox width height x y x1 x2 y1 y2 cx cy r rx ry d points fill fill-opacity fill-rule stroke stroke-width stroke-linecap stroke-linejoin stroke-miterlimit stroke-dasharray stroke-dashoffset stroke-opacity opacity transform clip-path clip-rule mask gradientUnits gradientTransform offset stop-color stop-opacity fx fy fr spreadMethod preserveAspectRatio version font-family font-size font-weight text-anchor dominant-baseline dx dy letter-spacing "
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	depth, roots := 0, 0
	for {
		offset := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			return roots == 1 && depth == 0
		}
		if err != nil {
			return false
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || t.Name.Local != "svg" {
					return false
				}
			}
			if (t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg") || !strings.Contains(elements, " "+t.Name.Local+" ") {
				return false
			}
			depth++
			for _, a := range t.Attr {
				if a.Name.Local == "xmlns" && a.Name.Space == "" && a.Value == "http://www.w3.org/2000/svg" {
					continue
				}
				if a.Name.Space != "" || !strings.Contains(attributes, " "+a.Name.Local+" ") {
					return false
				}
				value := strings.TrimSpace(a.Value)
				if strings.ContainsAny(value, "\\") {
					return false
				}
				if strings.Contains(strings.ToLower(value), "url") && !localSVGReference(value) {
					return false
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return false
			}
		case xml.Comment:
		case xml.ProcInst:
			if offset != 0 || t.Target != "xml" || !svgDeclaration.Match(t.Inst) {
				return false
			}
		default:
			return false
		}
	}
}

func localSVGReference(value string) bool {
	if !strings.HasPrefix(value, "url(#") || !strings.HasSuffix(value, ")") {
		return false
	}
	id := value[5 : len(value)-1]
	if id == "" {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
