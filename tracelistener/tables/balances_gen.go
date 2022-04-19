// This file was automatically generated. Please do not edit manually.

package tables

import (
	"fmt"
)

type BalancesTable struct {
	tableName string
}

func NewBalancesTable(tableName string) BalancesTable {
	return BalancesTable{
		tableName: tableName,
	}
}

func (r BalancesTable) CreateTable() string {
	return fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		(id serial PRIMARY KEY, height integer NOT NULL, delete_height integer, chain_name text NOT NULL, address text NOT NULL, amount text NOT NULL, denom text NOT NULL, UNIQUE (chain_name, address, denom))
	`, r.tableName)
}

func (r BalancesTable) Insert() string {
	return fmt.Sprintf(`
		INSERT INTO %s (height, chain_name, address, amount, denom)
		VALUES (:height, :chain_name, :address, :amount, :denom)
	`, r.tableName)
}

func (r BalancesTable) Upsert() string {
	return fmt.Sprintf(`
		INSERT INTO %s (height, chain_name, address, amount, denom)
		VALUES (:height, :chain_name, :address, :amount, :denom)
		ON CONFLICT (chain_name, address, denom)
		DO UPDATE
		SET height = EXCLUDED.height, chain_name = EXCLUDED.chain_name, address = EXCLUDED.address, amount = EXCLUDED.amount, denom = EXCLUDED.denom
	`, r.tableName)
}

func (r BalancesTable) Delete() string {
	return fmt.Sprintf(`
		DELETE FROM %s
		WHERE chain_name=:chain_name AND address=:address AND denom=:denom
	`, r.tableName)
}
