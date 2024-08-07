package exec

import (
	"sync"

	"github.com/hairyhenderson/gomplate/v3/data"
)

var (
	gomplateDatasourceFuncSyncMap = sync.Map{}
)

func gomplateDatasourceFunc(alias string, gomplateData *data.Data, args ...string) (any, error) {
	// If the result for the alias already exists in the cache, return it
	existingResult, found := gomplateDatasourceFuncSyncMap.Load(alias)
	if found && existingResult != nil {
		return existingResult, nil
	}

	result, err := gomplateData.Datasource(alias, args...)
	if err != nil {
		return nil, err
	}

	// Cache the result
	gomplateDatasourceFuncSyncMap.Store(alias, result)

	return result, nil
}
