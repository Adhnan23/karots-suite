package pharmacy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/customers"

	"github.com/jmoiron/sqlx"
)

// Store reads core (customers, products, sales, stock movements) and the
// Alternatives tables read-only, and writes only pharmacy_*.
type Store struct {
	db        *sqlx.DB
	customers *customers.Service
}

func NewStore(db *sqlx.DB) *Store { return &Store{db: db, customers: customers.NewService(db)} }

// Patient is a clinical profile on top of a core customer.
type Patient struct {
	ID         int64      `db:"id"`
	CustomerID int64      `db:"customer_id"`
	Name       string     `db:"name"`
	Phone      *string    `db:"phone"`
	DOB        *time.Time `db:"dob"`
	Sex        string     `db:"sex"`
	NIC        string     `db:"nic"`
	Allergies  string     `db:"allergies"`
	Notes      string     `db:"notes"`
}

const patientSelect = `
	SELECT p.id, p.customer_id, c.name, c.phone, p.dob, COALESCE(p.sex, '') AS sex, COALESCE(p.nic, '') AS nic,
	       p.allergies, p.notes
	FROM pharmacy_patients p JOIN customers c ON c.id = p.customer_id`

// Found is a search hit: a patient, or (PatientID 0) a customer not yet one.
type Found struct {
	PatientID  int64   `db:"patient_id"`
	CustomerID int64   `db:"customer_id"`
	Name       string  `db:"name"`
	Phone      *string `db:"phone"`
	NIC        string  `db:"nic"`
	Allergies  string  `db:"allergies"`
}

// Search finds patients by name / phone / NIC; withCustomers also lists matching
// customers who aren't patients yet (the till makes one a patient on pick).
func (s *Store) Search(ctx context.Context, q string, withCustomers bool, limit int) ([]Found, error) {
	out := []Found{}
	err := s.db.SelectContext(ctx, &out, `
		SELECT COALESCE(p.id, 0) AS patient_id, c.id AS customer_id, c.name, c.phone,
		       COALESCE(p.nic, '') AS nic, COALESCE(p.allergies, '') AS allergies
		FROM customers c LEFT JOIN pharmacy_patients p ON p.customer_id = c.id
		WHERE c.is_active AND (p.id IS NOT NULL OR ($2 AND $1 <> ''))
		  AND ($1 = '' OR c.name ILIKE '%' || $1 || '%' OR c.phone ILIKE '%' || $1 || '%' OR p.nic ILIKE '%' || $1 || '%')
		ORDER BY p.id IS NULL, c.name LIMIT $3`, strings.TrimSpace(q), withCustomers, limit)
	return out, err
}

func (s *Store) Patient(ctx context.Context, id int64) (Patient, error) {
	var p Patient
	err := s.db.GetContext(ctx, &p, patientSelect+` WHERE p.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return p, apperr.NotFound("patient")
	}
	return p, err
}

// FromCustomer makes a customer a patient (idempotent) and returns the profile.
func (s *Store) FromCustomer(ctx context.Context, customerID int64) (Patient, error) {
	if _, err := s.customers.Get(ctx, customerID); err != nil {
		return Patient{}, err
	}
	var id int64
	err := s.db.GetContext(ctx, &id, `
		WITH ins AS (INSERT INTO pharmacy_patients (customer_id) VALUES ($1) ON CONFLICT (customer_id) DO NOTHING RETURNING id)
		SELECT id FROM ins UNION ALL SELECT id FROM pharmacy_patients WHERE customer_id = $1 LIMIT 1`, customerID)
	if err != nil {
		return Patient{}, err
	}
	return s.Patient(ctx, id)
}

// Profile is what the pharmacy keeps beside the customer.
type Profile struct {
	DOB       *time.Time
	Sex, NIC  string
	Allergies string
	Notes     string
}

// Register makes a walk-in a customer (core reuses one with the same phone) and a
// patient, with their profile.
func (s *Store) Register(ctx context.Context, name, phone string, pr Profile) (Patient, error) {
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	if len(name) < 2 {
		return Patient{}, apperr.Validation("type the patient's name")
	}
	c, _, err := s.customers.Create(ctx, customers.CreateInput{Name: name, Phone: &phone})
	if err != nil {
		return Patient{}, err
	}
	p, err := s.FromCustomer(ctx, c.ID)
	if err != nil {
		return p, err
	}
	// A reused customer who is already a patient keeps what was on file unless typed now.
	if pr.Allergies == "" && pr.NIC == "" && pr.DOB == nil && pr.Sex == "" && pr.Notes == "" {
		return p, nil
	}
	if pr.Allergies == "" {
		pr.Allergies = p.Allergies
	}
	if pr.Notes == "" {
		pr.Notes = p.Notes
	}
	if err := s.UpdateProfile(ctx, p.ID, pr); err != nil {
		return p, err
	}
	return s.Patient(ctx, p.ID)
}

// UpdateProfile saves the pharmacy's profile (name and phone stay the core customer's).
func (s *Store) UpdateProfile(ctx context.Context, id int64, pr Profile) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pharmacy_patients SET dob = $2, sex = $3, nic = $4, allergies = $5, notes = $6 WHERE id = $1`,
		id, pr.DOB, strings.TrimSpace(pr.Sex), strings.TrimSpace(pr.NIC), strings.TrimSpace(pr.Allergies), strings.TrimSpace(pr.Notes))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("patient")
	}
	return nil
}
