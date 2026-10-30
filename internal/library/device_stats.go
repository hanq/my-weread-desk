package library

import "time"

// DeviceReadDay is one local calendar day of reading on this installation.
// WeRead only exposes account-wide totals, so this table is the source of
// truth for "this device" statistics.
type DeviceReadDay struct {
	Date    string `json:"date"`
	Seconds int64  `json:"seconds"`
}

// AddDeviceReadTime adds seconds to a local day. The reporter calls it on a
// short ticker, so writes are tiny and additive.
func (s *Store) AddDeviceReadTime(day string, seconds int64) error {
	if seconds <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO device_read_time(day, seconds, updated_at)
		VALUES (?,?,?) ON CONFLICT(day) DO UPDATE SET
		seconds=device_read_time.seconds+excluded.seconds,
		updated_at=excluded.updated_at`,
		day, seconds, time.Now().Unix())
	return err
}

// DeviceReadTotal returns all local reading seconds ever recorded.
func (s *Store) DeviceReadTotal() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(seconds),0) FROM device_read_time`).Scan(&total)
	return total, err
}

// DeviceReadTimes returns the most recent days in ascending order.
func (s *Store) DeviceReadTimes(days int) ([]DeviceReadDay, error) {
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT day, seconds FROM device_read_time
		WHERE day >= ? ORDER BY day ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeviceReadDay, 0, days)
	for rows.Next() {
		var row DeviceReadDay
		if err := rows.Scan(&row.Date, &row.Seconds); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
