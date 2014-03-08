// Copyright 2013-2014 Alexandre Fiori
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/xml"
	"log"
	"net"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	//_ "code.google.com/p/gosqlite/sqlite3"
)

type Cache struct {
	Country      map[string]string
	Region       map[RegionKey]string
	CityLocation map[uint32]Location
	CityBlock    BlockList
}

type RegionKey struct {
	CountryCode,
	RegionCode string
}

type Location struct {
	CountryCode,
	RegionCode,
	CityName,
	ZipCode string
	Latitude,
	Longitude float32
	MetroCode,
	AreaCode string
}

type Block struct {
	IpStart,
	IpEnd,
	LocId uint32
}

type BlockList []Block

func (p BlockList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p BlockList) Len() int {
	return len(p)
}

func (p BlockList) Less(i, j int) bool {
	return p[i].IpStart < p[j].IpStart
}

func NewCache(conf *ConfigFile) *Cache {
	db, err := sql.Open("sqlite3", conf.IPDB.File)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("PRAGMA cache_size=" + conf.IPDB.CacheSize)
	if err != nil {
		log.Fatal(err)
	}

	cache := &Cache{
		Country:      make(map[string]string),
		Region:       make(map[RegionKey]string),
		CityLocation: make(map[uint32]Location),
	}

	var row *sql.Rows

	// Load list of countries.
	if row, err = db.Query(`
		SELECT
			country_code,
			country_name
		FROM
			country_blocks
	`); err != nil {
		log.Fatal("Failed to load countries from db:", err)
	}

	var country_code, region_code, name string
	for row.Next() {
		if err = row.Scan(
			&country_code,
			&name,
		); err != nil {
			log.Fatal("Failed to load country from db:", err)
		}

		cache.Country[country_code] = name
	}

	row.Close()

	// Load list of regions.
	if row, err = db.Query(`
		SELECT
			country_code,
			region_code,
			region_name
		FROM
			region_names
	`); err != nil {
		log.Fatal("Failed to load regions from db:", err)
	}

	for row.Next() {
		if err = row.Scan(
			&country_code,
			&region_code,
			&name,
		); err != nil {
			log.Fatal("Failed to load region from db:", err)
		}

		cache.Region[RegionKey{country_code, region_code}] = name
	}

	row.Close()

	// Load list of city locations.
	if row, err = db.Query("SELECT * FROM city_location"); err != nil {
		log.Fatal("Failed to load city locations from db:", err)
	}

	var (
		locId uint32
		loc   Location
	)

	for row.Next() {
		if err = row.Scan(
			&locId,
			&loc.CountryCode,
			&loc.RegionCode,
			&loc.CityName,
			&loc.ZipCode,
			&loc.Latitude,
			&loc.Longitude,
			&loc.MetroCode,
			&loc.AreaCode,
		); err != nil {
			log.Fatal("Failed to load city location from db:", err)
		}

		cache.CityLocation[locId] = loc
	}

	row.Close()

	// Load list of city blocks.
	if row, err = db.Query("SELECT * from city_blocks"); err != nil {
		log.Fatal("Failed to load city blocks from db:", err)
	}

	var b Block
	for row.Next() {
		if err = row.Scan(&b.IpStart, &b.IpEnd, &b.LocId); err != nil {
			log.Fatal("Failed to load city block from db:", err)
		}

		cache.CityBlock = append(cache.CityBlock, b)
	}

	row.Close()

	sort.Sort(cache.CityBlock)

	return cache
}

func (cache *Cache) Query(IP net.IP, nIP uint32) *GeoIP {
	var reserved bool
	for _, net := range reservedIPs {
		if net.Contains(IP) {
			reserved = true
			break
		}
	}

	geoip := &GeoIP{Ip: IP.String()}
	if reserved {
		geoip.CountryCode = "RD"
		geoip.CountryName = "Reserved"
		return geoip
	}

	var n int
	for n = 0; n < len(cache.CityBlock); n++ {
		if cache.CityBlock[n].IpStart > nIP {
			break
		}
	}

	if n > 0 {
		n--
		if nIP <= cache.CityBlock[n].IpEnd {
			cache.Update(geoip, cache.CityBlock[n].LocId)
		}
	}

	return geoip
}

func (cache *Cache) Update(geoip *GeoIP, locId uint32) {
	city, ok := cache.CityLocation[locId]
	if !ok {
		return
	}

	geoip.CountryCode = city.CountryCode
	geoip.CountryName = cache.Country[city.CountryCode]

	geoip.RegionCode = city.RegionCode
	geoip.RegionName = cache.Region[RegionKey{
		city.CountryCode,
		city.RegionCode,
	}]

	geoip.CityName = city.CityName
	geoip.ZipCode = city.ZipCode
	geoip.Latitude = city.Latitude
	geoip.Longitude = city.Longitude
	geoip.MetroCode = city.MetroCode
	geoip.AreaCode = city.AreaCode
}

type GeoIP struct {
	XMLName     xml.Name `json:"-" xml:"Response"`
	Ip          string   `json:"ip"`
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	RegionCode  string   `json:"region_code"`
	RegionName  string   `json:"region_name"`
	CityName    string   `json:"city" xml:"City"`
	ZipCode     string   `json:"zipcode"`
	Latitude    float32  `json:"latitude"`
	Longitude   float32  `json:"longitude"`
	MetroCode   string   `json:"metro_code"`
	AreaCode    string   `json:"areacode"`
}

// http://en.wikipedia.org/wiki/Reserved_IP_addresses
var reservedIPs = []net.IPNet{
	{net.IPv4(0, 0, 0, 0), net.IPv4Mask(255, 0, 0, 0)},
	{net.IPv4(10, 0, 0, 0), net.IPv4Mask(255, 0, 0, 0)},
	{net.IPv4(100, 64, 0, 0), net.IPv4Mask(255, 192, 0, 0)},
	{net.IPv4(127, 0, 0, 0), net.IPv4Mask(255, 0, 0, 0)},
	{net.IPv4(169, 254, 0, 0), net.IPv4Mask(255, 255, 0, 0)},
	{net.IPv4(172, 16, 0, 0), net.IPv4Mask(255, 240, 0, 0)},
	{net.IPv4(192, 0, 0, 0), net.IPv4Mask(255, 255, 255, 248)},
	{net.IPv4(192, 0, 2, 0), net.IPv4Mask(255, 255, 255, 0)},
	{net.IPv4(192, 88, 99, 0), net.IPv4Mask(255, 255, 255, 0)},
	{net.IPv4(192, 168, 0, 0), net.IPv4Mask(255, 255, 0, 0)},
	{net.IPv4(198, 18, 0, 0), net.IPv4Mask(255, 254, 0, 0)},
	{net.IPv4(198, 51, 100, 0), net.IPv4Mask(255, 255, 255, 0)},
	{net.IPv4(203, 0, 113, 0), net.IPv4Mask(255, 255, 255, 0)},
	{net.IPv4(224, 0, 0, 0), net.IPv4Mask(240, 0, 0, 0)},
	{net.IPv4(240, 0, 0, 0), net.IPv4Mask(240, 0, 0, 0)},
	{net.IPv4(255, 255, 255, 255), net.IPv4Mask(255, 255, 255, 255)},
}
