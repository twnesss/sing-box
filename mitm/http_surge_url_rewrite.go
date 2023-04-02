package mitm

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

type HTTPHandlerFunc func(writer http.ResponseWriter, request *http.Request, urlString string) bool

func readSurgeURLRewriteRules(file *os.File) ([]HTTPHandlerFunc, error) {
	defer file.Close()
	reader := bufio.NewReader(file)
	var handlers []HTTPHandlerFunc
	for {
		lineBytes, _, err := reader.ReadLine()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		ruleLine := strings.TrimSpace(string(lineBytes))
		if ruleLine == "" || ruleLine[0] == '#' {
			continue
		}
		ruleParts := strings.Split(ruleLine, " ")
		if len(ruleParts) != 3 {
			return nil, E.New("invalid surge url rewrite line: ", ruleLine)
		}
		urlRegex, err := regexp.Compile(ruleParts[0])
		if err != nil {
			return nil, E.Cause(err, "invalid surge url rewrite line (bad regex): ", ruleLine)
		}
		switch ruleParts[2] {
		case "reject":
			handlers = append(handlers, surgeURLRewriteReject(urlRegex))
		case "header":
			// TODO: support header redirect
			fallthrough
		case "302":
			handlers = append(handlers, surgeURLRewrite302(urlRegex, ruleParts[1]))
		default:
			return nil, E.Cause(err, "invalid surge url rewrite line (unknown acton): ", ruleLine)
		}
	}
	return handlers, nil
}

func surgeURLRewriteReject(urlRegex *regexp.Regexp) HTTPHandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request, urlString string) bool {
		if !urlRegex.MatchString(urlString) {
			return false
		}
		writer.WriteHeader(404)
		return true
	}
}

func surgeURLRewrite302(urlRegex *regexp.Regexp, rewriteURL string) HTTPHandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request, urlString string) bool {
		if !urlRegex.MatchString(urlString) {
			return false
		}
		// use 307 to keep method
		http.RedirectHandler(rewriteURL, http.StatusTemporaryRedirect).ServeHTTP(writer, request)
		return true
	}
}
