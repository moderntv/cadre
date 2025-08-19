package file

import (
	"testing"

	"github.com/moderntv/cadre/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	aggregatorService = "aggregator"
	ingestService     = "ingest"
	apiService        = "api"
)

func TestFileRegistry(t *testing.T) {
	fr, err := NewRegistry("./testdata/registry.yaml")
	require.NoError(t, err)

	t.Run("Instances_AggregatorService_ReturnsServiceInstances", func(t *testing.T) {
		instances := fr.Instances(ingestService)
		assert.Len(t, instances, 1)
		assert.Equal(t,
			[]registry.Instance{
				&instance{serviceName: ingestService, addr: "ingest.moderntv.eu"},
			},
			instances,
		)
	})

	t.Run("Instances_IngestService_ReturnsServiceInstances", func(t *testing.T) {
		instances := fr.Instances(aggregatorService)
		assert.Len(t, instances, 3)
		assert.Equal(t,
			[]registry.Instance{
				&instance{serviceName: aggregatorService, addr: "aggregator1.moderntv.eu"},
				&instance{serviceName: aggregatorService, addr: "aggregator2.moderntv.eu"},
				&instance{serviceName: aggregatorService, addr: "aggregator3.moderntv.eu"},
			},
			instances,
		)
	})

	t.Run("Instances_APIService_ReturnsNoInstances", func(t *testing.T) {
		instances := fr.Instances(apiService)
		assert.Empty(t, instances)
	})

	t.Run("Watch_StopMultipleWatchersForSingleService_CorrectlyCleanupsWatchers", func(t *testing.T) {
		_, stop1 := fr.Watch(ingestService)
		_, stop2 := fr.Watch(ingestService)
		_, stop3 := fr.Watch(ingestService)

		fr := fr.(*fileRegistry)

		assert.Len(t, fr.watchers[ingestService], 3)

		stop1()
		assert.Len(t, fr.watchers[ingestService], 2)

		stop2()
		assert.Len(t, fr.watchers[ingestService], 1)

		stop3()
		assert.Empty(t, fr.watchers[ingestService])
	})
}
