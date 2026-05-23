package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

/*
========================================================
INPUT STRUCTURE (ФАЙЛ)
========================================================
*/

type KnownAddressFile struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Source  string `json:"source"`
}

/*
========================================================
ENTRY POINT (ТИ ВИКЛИКАЄШ У main)
========================================================
*/

func LoadKnownAddressesIntoDB(filePath string) error {

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var raw map[string]KnownAddressFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for addr, v := range raw {
		address := strings.ToLower(addr)

		exists, err := DB_AddressExists(address)
		if err != nil {
			log.Println("exists err:", err)
			continue
		}

		class, conf, rule, err := DB_MatchRule(v.Name)
		if err != nil {
			log.Println("match rule err:", err)
			continue
		}

		if !exists {
			_ = DB_InsertAddress(
				address,
				v.Name,
				class,
				conf,
				v.Source,
				rule,
			)
		} else {
			_ = DB_UpdateAddress(
				address,
				class,
				conf,
				rule,
			)
		}
	}

	return nil
}

/*
========================================================
DB HELPERS (WRITE + RULES)
========================================================
*/
