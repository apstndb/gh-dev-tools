package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
)

const graphQLRateLimitAlias = "ghHelperApiUsageRateLimit"

type graphQLRateLimitTelemetry struct {
	Cost      int    `json:"cost"`
	Limit     int    `json:"limit"`
	NodeCount int    `json:"nodeCount"`
	Remaining int    `json:"remaining"`
	Used      int    `json:"used"`
	ResetAt   string `json:"resetAt"`
}

func injectGraphQLRateLimit(query string) (string, bool) {
	if strings.Contains(query, graphQLRateLimitAlias) {
		return query, true
	}

	operationStart, ok := findGraphQLOperationStart(query)
	if !ok {
		return query, false
	}

	operation := readGraphQLName(query, operationStart)
	if operation == "mutation" || operation == "subscription" {
		return query, false
	}

	var openBrace int
	if operation == "" && query[operationStart] == '{' {
		openBrace = operationStart
	} else if operation == "query" {
		openBrace = findNextGraphQLSelectionBrace(query, operationStart+len(operation))
	} else {
		return query, false
	}
	if openBrace < 0 {
		return query, false
	}

	closeBrace := findMatchingGraphQLBrace(query, openBrace)
	if closeBrace < 0 {
		return query, false
	}

	injected := "\n  " + graphQLRateLimitAlias + ": rateLimit {\n    cost\n    limit\n    nodeCount\n    remaining\n    used\n    resetAt\n  }\n"
	return query[:closeBrace] + injected + query[closeBrace:], true
}

func isGraphQLQueryOperation(query string) bool {
	operationStart, ok := findGraphQLOperationStart(query)
	if !ok {
		return false
	}
	if query[operationStart] == '{' {
		return true
	}
	return readGraphQLName(query, operationStart) == "query"
}

func graphQLRateLimitFromResponse(body []byte) (graphQLRateLimitTelemetry, bool) {
	var response struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return graphQLRateLimitTelemetry{}, false
	}
	raw, ok := response.Data[graphQLRateLimitAlias]
	if !ok {
		return graphQLRateLimitTelemetry{}, false
	}
	var rateLimit graphQLRateLimitTelemetry
	if err := json.Unmarshal(raw, &rateLimit); err != nil {
		return graphQLRateLimitTelemetry{}, false
	}
	return rateLimit, true
}

func findGraphQLOperationStart(query string) (int, bool) {
	scanner := graphQLScanner{input: query}
	for {
		token, pos, ok := scanner.nextToken()
		if !ok {
			return 0, false
		}
		switch token {
		case "{":
			return pos, true
		case "query", "mutation", "subscription":
			return pos, true
		case "fragment":
			if !scanner.skipFragmentDefinition() {
				return 0, false
			}
		}
	}
}

func findNextGraphQLSelectionBrace(query string, start int) int {
	scanner := graphQLScanner{input: query, pos: start}
	for {
		token, pos, ok := scanner.nextToken()
		if !ok {
			return -1
		}
		if token == "{" {
			return pos
		}
	}
}

func findMatchingGraphQLBrace(query string, openBrace int) int {
	scanner := graphQLScanner{input: query, pos: openBrace}
	depth := 0
	for {
		token, pos, ok := scanner.nextToken()
		if !ok {
			return -1
		}
		switch token {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				return pos
			}
		}
	}
}

func readGraphQLName(query string, pos int) string {
	end := pos
	for end < len(query) {
		r := rune(query[end])
		if !isGraphQLNameRune(r) {
			break
		}
		end++
	}
	return query[pos:end]
}

type graphQLScanner struct {
	input string
	pos   int
}

func (s *graphQLScanner) nextToken() (string, int, bool) {
	for s.pos < len(s.input) {
		r := rune(s.input[s.pos])
		if unicode.IsSpace(r) || r == ',' {
			s.pos++
			continue
		}
		if r == '#' {
			s.skipLineComment()
			continue
		}
		if r == '"' {
			if strings.HasPrefix(s.input[s.pos:], `"""`) {
				s.skipBlockString()
			} else {
				s.skipString()
			}
			continue
		}
		if r == '{' || r == '}' {
			pos := s.pos
			s.pos++
			return string(r), pos, true
		}
		if isGraphQLNameRune(r) {
			start := s.pos
			for s.pos < len(s.input) && isGraphQLNameRune(rune(s.input[s.pos])) {
				s.pos++
			}
			return s.input[start:s.pos], start, true
		}
		s.pos++
	}
	return "", 0, false
}

func (s *graphQLScanner) skipFragmentDefinition() bool {
	depth := 0
	seenOpen := false
	for {
		token, _, ok := s.nextToken()
		if !ok {
			return false
		}
		switch token {
		case "{":
			depth++
			seenOpen = true
		case "}":
			depth--
			if seenOpen && depth == 0 {
				return true
			}
		}
	}
}

func (s *graphQLScanner) skipLineComment() {
	for s.pos < len(s.input) && s.input[s.pos] != '\n' {
		s.pos++
	}
}

func (s *graphQLScanner) skipString() {
	s.pos++
	for s.pos < len(s.input) {
		if s.input[s.pos] == '\\' {
			s.pos += 2
			continue
		}
		if s.input[s.pos] == '"' {
			s.pos++
			return
		}
		s.pos++
	}
}

func (s *graphQLScanner) skipBlockString() {
	s.pos += 3
	if end := bytes.Index([]byte(s.input[s.pos:]), []byte(`"""`)); end >= 0 {
		s.pos += end + 3
		return
	}
	s.pos = len(s.input)
}

func isGraphQLNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
