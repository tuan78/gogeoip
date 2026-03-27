package geo

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Data holds the geolocation information for an IP address.
type Data struct {
	IP            string `json:"ip"`
	CountryCode   string `json:"country_code"`
	CountryName   string `json:"country_name"`
	ContinentCode string `json:"continent_code"`
	ContinentName string `json:"continent_name"`
}

// GeoReader is an interface for geolocation reader operations.
type GeoReader interface {
	Country(ip net.IP) (*geoip2.Country, error)
	Close() error
}

// Lookup returns the geolocation data for the given IP address using the DB.
// Returns an error if the database is not loaded or the lookup fails.
func (d *DB) Lookup(ipStr string) (*Data, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	reader, unlock := d.Reader()
	if reader == nil {
		unlock()
		return nil, fmt.Errorf("MaxMind database not loaded")
	}
	record, err := reader.Country(ip)
	unlock()
	if err != nil {
		return nil, fmt.Errorf("MaxMind lookup failed: %w", err)
	}

	// Prefer English names; fall back to empty string if not present.
	countryName := record.Country.Names["en"]
	continentName := record.Continent.Names["en"]

	return &Data{
		IP:            ipStr,
		CountryCode:   record.Country.IsoCode,
		CountryName:   countryName,
		ContinentCode: record.Continent.Code,
		ContinentName: continentName,
	}, nil
}
