//go:build plugins
// +build plugins

package boltz

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dbpath = "./test.db"

type Dummy struct {
	Name string
}

type ExtendedJobData struct {
	Burek string
	JobData
}

func TestBoltzDB(t *testing.T) {
	os.Remove(dbpath)
	db := &BoltzDB{}
	err := db.Connect(dbpath)
	require.NoError(t, err)
	defer db.db.Close()
	defer os.Remove(dbpath)

	t.Run("Store and retrieve data", func(t *testing.T) {
		err := db.Insert("key", &Dummy{Name: "str"})
		require.NoError(t, err)

		err = db.Insert(123, &Dummy{Name: "int"})
		require.NoError(t, err)

		var d Dummy
		err = db.Get("key", &d)
		require.NoError(t, err)
		require.Equal(t, "str", d.Name)

		err = db.Get(123, &d)
		require.NoError(t, err)
		require.Equal(t, "int", d.Name)
	})

	t.Run("Error on not found", func(t *testing.T) {
		var d Dummy
		err := db.Get("notfound", &d)
		require.EqualError(t, err, "No data found for this key")
	})
	t.Run("Extended entities", func(t *testing.T) {
		jd := &JobData{ID: 1337}
		err = db.Insert(1337, jd)
		assert.NoError(t, err)

		var jd2 JobData
		err = db.Get(1337, &jd2)
		assert.NoError(t, err)

		assert.Equal(t, int32(1337), jd.ID)

		ejd := &ExtendedJobData{Burek: "mesni", JobData: *jd}
		err = db.Insert(1337, ejd)
		assert.NoError(t, err)

		//err = db.Get(1337, &jd2)
		//assert.NoError(t, err)
	})
}
