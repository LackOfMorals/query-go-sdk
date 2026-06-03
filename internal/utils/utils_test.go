package utils

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestBase64Encode verifies base64 encoding for Basic Auth
func TestBase64Encode(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected string
	}{
		{
			name:     "simple credentials",
			s1:       "user",
			s2:       "pass",
			expected: base64.StdEncoding.EncodeToString([]byte("user:pass")),
		},
		{
			name:     "complex credentials",
			s1:       "client-id-123",
			s2:       "secret-key-456",
			expected: base64.StdEncoding.EncodeToString([]byte("client-id-123:secret-key-456")),
		},
		{
			name:     "empty strings",
			s1:       "",
			s2:       "",
			expected: base64.StdEncoding.EncodeToString([]byte(":")),
		},
		{
			name:     "special characters",
			s1:       "user@domain",
			s2:       "p@$$w0rd!",
			expected: base64.StdEncoding.EncodeToString([]byte("user@domain:p@$$w0rd!")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Base64Encode(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}

			// Verify it can be decoded back
			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("failed to decode result: %v", err)
			}
			expectedDecoded := tt.s1 + ":" + tt.s2
			if string(decoded) != expectedDecoded {
				t.Errorf("decoded value '%s' doesn't match expected '%s'", string(decoded), expectedDecoded)
			}
		})
	}
}

// TestUnmarshal_SimpleStruct verifies unmarshaling JSON into simple struct
func TestUnmarshal_SimpleStruct(t *testing.T) {
	type SimpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := []byte(`{"name":"test","value":42}`)

	result, err := Unmarshal[SimpleStruct](jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("expected value 42, got %d", result.Value)
	}
}

// TestUnmarshal_NestedStruct verifies unmarshaling nested structures
func TestUnmarshal_NestedStruct(t *testing.T) {
	type Inner struct {
		ID string `json:"id"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	jsonData := []byte(`{"name":"outer","inner":{"id":"inner-id"}}`)

	result, err := Unmarshal[Outer](jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "outer" {
		t.Errorf("expected name 'outer', got '%s'", result.Name)
	}
	if result.Inner.ID != "inner-id" {
		t.Errorf("expected inner id 'inner-id', got '%s'", result.Inner.ID)
	}
}

// TestUnmarshal_Array verifies unmarshaling JSON arrays
func TestUnmarshal_Array(t *testing.T) {
	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	jsonData := []byte(`[{"id":"1","name":"first"},{"id":"2","name":"second"}]`)

	result, err := Unmarshal[[]Item](jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != "1" {
		t.Errorf("expected first id '1', got '%s'", result[0].ID)
	}
	if result[1].Name != "second" {
		t.Errorf("expected second name 'second', got '%s'", result[1].Name)
	}
}

// TestUnmarshal_InvalidJSON verifies error handling for invalid JSON
func TestUnmarshal_InvalidJSON(t *testing.T) {
	type SimpleStruct struct {
		Name string `json:"name"`
	}

	invalidJSON := []byte(`{"name": invalid}`)

	_, err := Unmarshal[SimpleStruct](invalidJSON)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestMarshall_SimpleStruct verifies marshaling simple struct to JSON
func TestMarshall_SimpleStruct(t *testing.T) {
	type SimpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := SimpleStruct{
		Name:  "test",
		Value: 42,
	}

	result, err := Marshal(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unmarshal to verify
	var unmarshaled SimpleStruct
	if err := json.Unmarshal(result, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if unmarshaled.Name != data.Name {
		t.Errorf("expected name '%s', got '%s'", data.Name, unmarshaled.Name)
	}
	if unmarshaled.Value != data.Value {
		t.Errorf("expected value %d, got %d", data.Value, unmarshaled.Value)
	}
}

// TestMarshallIndent_Formatting verifies indented JSON output
func TestMarshallIndent_Formatting(t *testing.T) {
	type SimpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := SimpleStruct{
		Name:  "test",
		Value: 42,
	}

	result, err := MarshalIndent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var unmarshaled SimpleStruct
	if err := json.Unmarshal(result, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Verify indentation (result should contain newlines and spaces)
	resultStr := string(result)
	if !contains(resultStr, "\n") {
		t.Error("expected newlines in indented JSON")
	}
	if !contains(resultStr, "  ") {
		t.Error("expected indentation in JSON")
	}
}

// TestCheckDate_ValidDates verifies valid date formats
func TestCheckDate_ValidDates(t *testing.T) {
	validDates := []string{
		"2024-01-15",
		"2024-12-31",
		"2023-06-30",
		"2025-03-01",
	}

	for _, date := range validDates {
		t.Run(date, func(t *testing.T) {
			err := CheckDate(date)
			if err != nil {
				t.Errorf("expected no error for valid date '%s', got %v", date, err)
			}
		})
	}
}

// TestCheckDate_InvalidDates verifies invalid date formats
func TestCheckDate_InvalidDates(t *testing.T) {
	invalidDates := []string{
		"2024/01/15",          // Wrong separator
		"15-01-2024",          // Wrong order
		"2024-1-15",           // Missing leading zero
		"2024-01-32",          // Invalid day
		"2024-13-01",          // Invalid month
		"24-01-15",            // Wrong year format
		"2024-01",             // Incomplete
		"not-a-date",          // Not a date
		"",                    // Empty
		"2024-01-15T00:00:00", // With time
	}

	for _, date := range invalidDates {
		t.Run(date, func(t *testing.T) {
			err := CheckDate(date)
			if err == nil {
				t.Errorf("expected error for invalid date '%s', got nil", date)
			}
		})
	}
}

// TestCheckDate_ErrorMessage verifies error message format
func TestCheckDate_ErrorMessage(t *testing.T) {
	err := CheckDate("invalid-date")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedMsg := "the date must in the format of YYYY-MM-DD"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestCheckDate_EdgeCases verifies edge case dates
func TestCheckDate_EdgeCases(t *testing.T) {
	tests := []struct {
		date      string
		shouldErr bool
	}{
		{"2024-02-29", false}, // Leap year
		{"2023-02-29", true},  // Not a leap year
		{"2024-01-01", false}, // Year start
		{"2024-12-31", false}, // Year end
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			err := CheckDate(tt.date)
			if tt.shouldErr && err == nil {
				t.Errorf("expected error for date '%s', got nil", tt.date)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("expected no error for date '%s', got %v", tt.date, err)
			}
		})
	}
}

// BenchmarkBase64Encode benchmarks base64 encoding
func BenchmarkBase64Encode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Base64Encode("client-id", "client-secret")
	}
}

// BenchmarkUnmarshal benchmarks JSON unmarshaling
func BenchmarkUnmarshal(b *testing.B) {
	type SimpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := []byte(`{"name":"test","value":42}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Unmarshal[SimpleStruct](jsonData)
	}
}

// BenchmarkMarshall benchmarks JSON marshaling
func BenchmarkMarshall(b *testing.B) {
	type SimpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := SimpleStruct{Name: "test", Value: 42}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(data)
	}
}

// BenchmarkCheckDate benchmarks date validation
func BenchmarkCheckDate(b *testing.B) {
	date := "2024-01-15"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CheckDate(date)
	}
}

// TestTruncateString verifies correct truncation by rune count, not byte count.
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate ASCII", "hello world", 5, "hello"},
		{"empty string", "", 5, ""},
		{"zero limit", "hello", 0, ""},
		// Multibyte: each of these characters is 3 bytes in UTF-8.
		// Byte-slicing at n=3 would return only the first character;
		// rune-slicing at n=3 returns the first three characters.
		{"multibyte rune count", "日本語テスト", 3, "日本語"},
		{"multibyte no truncation", "日本語", 10, "日本語"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
