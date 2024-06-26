package migration_test

import (
	"encoding/gob"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/netbirdio/netbird/management/server"
	"github.com/netbirdio/netbird/management/server/migration"
	"github.com/netbirdio/netbird/route"
)

func setupDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		PrepareStmt: true,
	})

	require.NoError(t, err, "Failed to open database")
	return db
}

func TestMigrateFieldFromGobToJSON_EmptyDB(t *testing.T) {
	db := setupDatabase(t)
	err := migration.MigrateFieldFromGobToJSON[server.Account, net.IPNet](db, "network_net")
	require.NoError(t, err, "Migration should not fail for an empty database")
}

func TestMigrateFieldFromGobToJSON_WithGobData(t *testing.T) {
	db := setupDatabase(t)

	err := db.AutoMigrate(&server.Account{}, &route.Route{})
	require.NoError(t, err, "Failed to auto-migrate tables")

	_, ipnet, err := net.ParseCIDR("10.0.0.0/24")
	require.NoError(t, err, "Failed to parse CIDR")

	type network struct {
		server.Network
		Net net.IPNet `gorm:"serializer:gob"`
	}

	type account struct {
		server.Account
		Network *network `gorm:"embedded;embeddedPrefix:network_"`
	}

	err = db.Save(&account{Account: server.Account{Id: "123"}, Network: &network{Net: *ipnet}}).Error
	require.NoError(t, err, "Failed to insert Gob data")

	var gobStr string
	err = db.Model(&server.Account{}).Select("network_net").First(&gobStr).Error
	assert.NoError(t, err, "Failed to fetch Gob data")

	err = gob.NewDecoder(strings.NewReader(gobStr)).Decode(&ipnet)
	require.NoError(t, err, "Failed to decode Gob data")

	err = migration.MigrateFieldFromGobToJSON[server.Account, net.IPNet](db, "network_net")
	require.NoError(t, err, "Migration should not fail with Gob data")

	var jsonStr string
	db.Model(&server.Account{}).Select("network_net").First(&jsonStr)
	assert.JSONEq(t, `{"IP":"10.0.0.0","Mask":"////AA=="}`, jsonStr, "Data should be migrated")
}

func TestMigrateFieldFromGobToJSON_WithJSONData(t *testing.T) {
	db := setupDatabase(t)

	err := db.AutoMigrate(&server.Account{}, &route.Route{})
	require.NoError(t, err, "Failed to auto-migrate tables")

	_, ipnet, err := net.ParseCIDR("10.0.0.0/24")
	require.NoError(t, err, "Failed to parse CIDR")

	err = db.Save(&server.Account{Network: &server.Network{Net: *ipnet}}).Error
	require.NoError(t, err, "Failed to insert JSON data")

	err = migration.MigrateFieldFromGobToJSON[server.Account, net.IPNet](db, "network_net")
	require.NoError(t, err, "Migration should not fail with JSON data")

	var jsonStr string
	db.Model(&server.Account{}).Select("network_net").First(&jsonStr)
	assert.JSONEq(t, `{"IP":"10.0.0.0","Mask":"////AA=="}`, jsonStr, "Data should be unchanged")
}
