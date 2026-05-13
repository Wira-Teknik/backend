package utils

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// JSONDate adalah tipe custom untuk time.Time agar serialisasi JSON menggunakan format YYYY-MM-DD.
type JSONDate time.Time

// MarshalJSON mengonversi time.Time ke string "YYYY-MM-DD".
func (j JSONDate) MarshalJSON() ([]byte, error) {
	t := time.Time(j)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Format("2006-01-02"))), nil
}

// Value mengimplementasikan driver.Valuer agar GORM bisa menyimpan ke database.
func (j JSONDate) Value() (driver.Value, error) {
	t := time.Time(j)
	if t.IsZero() {
		return nil, nil
	}
	return t, nil
}

// Scan mengimplementasikan sql.Scanner agar GORM bisa membaca dari database.
func (j *JSONDate) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("tipe data tidak valid untuk JSONDate: %T", value)
	}
	*j = JSONDate(t)
	return nil
}

// JSONDateTime adalah tipe custom untuk time.Time agar serialisasi JSON menggunakan format YYYY-MM-DD HH:MM:SS.
type JSONDateTime time.Time

// MarshalJSON mengonversi time.Time ke string "YYYY-MM-DD HH:MM:SS".
func (j JSONDateTime) MarshalJSON() ([]byte, error) {
	t := time.Time(j)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Format("2006-01-02 15:04:05"))), nil
}

// Value mengimplementasikan driver.Valuer.
func (j JSONDateTime) Value() (driver.Value, error) {
	t := time.Time(j)
	if t.IsZero() {
		return nil, nil
	}
	return t, nil
}

// Scan mengimplementasikan sql.Scanner.
func (j *JSONDateTime) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("tipe data tidak valid untuk JSONDateTime: %T", value)
	}
	*j = JSONDateTime(t)
	return nil
}
