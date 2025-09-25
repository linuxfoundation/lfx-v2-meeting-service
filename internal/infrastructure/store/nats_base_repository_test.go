// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
)

// TestEntity for testing the base repository
type TestEntity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestNatsBaseRepository_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		kvStore  INatsKeyValue
		expected bool
	}{
		{
			name:     "ready when kvStore is not nil",
			kvStore:  newMockNatsKeyValue(),
			expected: true,
		},
		{
			name:     "not ready when kvStore is nil",
			kvStore:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewNatsBaseRepository[TestEntity](tt.kvStore, "test")
			assert.Equal(t, tt.expected, repo.IsReady())
		})
	}
}

func TestNatsBaseRepository_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		entity := &TestEntity{ID: "test-1", Name: "Test Entity"}
		entityJSON, _ := json.Marshal(entity)
		mockKV.data["test-key"] = entityJSON
		mockKV.revisions["test-key"] = 1

		result, err := repo.Get(ctx, "test-key")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, entity.ID, result.ID)
		assert.Equal(t, entity.Name, result.Name)
	})

	t.Run("not found", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		result, err := repo.Get(ctx, "nonexistent")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		result, err := repo.Get(ctx, "test-key")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_GetWithRevision(t *testing.T) {
	ctx := context.Background()
	mockKV := newMockNatsKeyValue()
	repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

	entity := &TestEntity{ID: "test-1", Name: "Test Entity"}
	entityJSON, _ := json.Marshal(entity)
	mockKV.data["test-key"] = entityJSON
	mockKV.revisions["test-key"] = 5

	result, revision, err := repo.GetWithRevision(ctx, "test-key")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, entity.ID, result.ID)
	assert.Equal(t, uint64(5), revision)
}

func TestNatsBaseRepository_Create(t *testing.T) {
	ctx := context.Background()
	entity := &TestEntity{ID: "test-1", Name: "Test Entity"}

	t.Run("successful create", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		err := repo.Create(ctx, "test-key", entity)

		assert.NoError(t, err)

		// Verify data was stored
		storedData, exists := mockKV.data["test-key"]
		assert.True(t, exists)

		var storedEntity TestEntity
		err = json.Unmarshal(storedData, &storedEntity)
		assert.NoError(t, err)
		assert.Equal(t, entity.ID, storedEntity.ID)
		assert.Equal(t, entity.Name, storedEntity.Name)
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		err := repo.Create(ctx, "test-key", entity)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})

	t.Run("put error", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		mockKV.putError = jetstream.ErrKeyNotFound // Any error
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		err := repo.Create(ctx, "test-key", entity)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_Update(t *testing.T) {
	ctx := context.Background()
	entity := &TestEntity{ID: "test-1", Name: "Updated Entity"}

	t.Run("successful update", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		// Setup existing data
		originalEntity := &TestEntity{ID: "test-1", Name: "Original Entity"}
		originalJSON, _ := json.Marshal(originalEntity)
		mockKV.data["test-key"] = originalJSON
		mockKV.revisions["test-key"] = 1

		err := repo.Update(ctx, "test-key", entity, 1)

		assert.NoError(t, err)

		// Verify data was updated
		storedData := mockKV.data["test-key"]
		var storedEntity TestEntity
		err = json.Unmarshal(storedData, &storedEntity)
		assert.NoError(t, err)
		assert.Equal(t, entity.Name, storedEntity.Name)
	})

	t.Run("not found", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		err := repo.Update(ctx, "nonexistent", entity, 1)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
	})

	t.Run("revision conflict", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		// Setup existing data with different revision
		originalEntity := &TestEntity{ID: "test-1", Name: "Original Entity"}
		originalJSON, _ := json.Marshal(originalEntity)
		mockKV.data["test-key"] = originalJSON
		mockKV.revisions["test-key"] = 5

		err := repo.Update(ctx, "test-key", entity, 1) // Wrong revision

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeConflict, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		// Setup existing data
		mockKV.data["test-key"] = []byte(`{"id":"test-1"}`)
		mockKV.revisions["test-key"] = 1

		err := repo.Delete(ctx, "test-key", 1)

		assert.NoError(t, err)

		// Verify data was deleted
		_, exists := mockKV.data["test-key"]
		assert.False(t, exists)
	})

	t.Run("not found", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		err := repo.Delete(ctx, "nonexistent", 1)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		err := repo.Delete(ctx, "test-key", 1)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_Exists(t *testing.T) {
	ctx := context.Background()

	t.Run("exists", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		entity := &TestEntity{ID: "test-1", Name: "Test Entity"}
		entityJSON, _ := json.Marshal(entity)
		mockKV.data["test-key"] = entityJSON
		mockKV.revisions["test-key"] = 1

		exists, err := repo.Exists(ctx, "test-key")

		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("does not exist", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		exists, err := repo.Exists(ctx, "nonexistent")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestNatsBaseRepository_ListEntities(t *testing.T) {
	ctx := context.Background()

	t.Run("list with pattern matching", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		entity1 := &TestEntity{ID: "test-1", Name: "Test Entity 1"}
		entity2 := &TestEntity{ID: "test-2", Name: "Test Entity 2"}
		entity3 := &TestEntity{ID: "other-1", Name: "Other Entity"}

		entity1JSON, _ := json.Marshal(entity1)
		entity2JSON, _ := json.Marshal(entity2)
		entity3JSON, _ := json.Marshal(entity3)

		mockKV.data["test/test-1"] = entity1JSON
		mockKV.data["test/test-2"] = entity2JSON
		mockKV.data["other/other-1"] = entity3JSON

		entities, err := repo.ListEntities(ctx, "test/")

		assert.NoError(t, err)
		assert.Len(t, entities, 2) // Only entities with keys containing "test/"
	})

	t.Run("list all entities (empty pattern)", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		entity := &TestEntity{ID: "test-1", Name: "Test Entity"}
		entityJSON, _ := json.Marshal(entity)
		mockKV.data["test-key"] = entityJSON

		entities, err := repo.ListEntities(ctx, "")

		assert.NoError(t, err)
		assert.Len(t, entities, 1)
		assert.Equal(t, entity.ID, entities[0].ID)
	})
}

func TestNatsBaseRepository_ListEntitiesEncoded(t *testing.T) {
	ctx := context.Background()
	kb := NewKeyBuilder("")

	t.Run("list encoded entities with pattern", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		entity1 := &TestEntity{ID: "test-1", Name: "Test Entity 1"}
		entity2 := &TestEntity{ID: "other-1", Name: "Other Entity"}

		entity1JSON, _ := json.Marshal(entity1)
		entity2JSON, _ := json.Marshal(entity2)

		// Create encoded keys
		encodedKey1, _ := kb.EncodeKey("registrant/test-1")
		encodedKey2, _ := kb.EncodeKey("meeting/other-1")

		mockKV.data[encodedKey1] = entity1JSON
		mockKV.data[encodedKey2] = entity2JSON

		entities, err := repo.ListEntitiesEncoded(ctx, "registrant/", kb)

		assert.NoError(t, err)
		assert.Len(t, entities, 1) // Only the registrant entity should match
		assert.Equal(t, entity1.ID, entities[0].ID)
	})
}

func TestNatsBaseRepository_PutIndex(t *testing.T) {
	ctx := context.Background()

	t.Run("successful put index", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		err := repo.PutIndex(ctx, "index-key")

		assert.NoError(t, err)

		// Verify empty data was stored
		storedData, exists := mockKV.data["index-key"]
		assert.True(t, exists)
		assert.Equal(t, []byte{}, storedData)
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		err := repo.PutIndex(ctx, "index-key")

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_DeleteIndex(t *testing.T) {
	ctx := context.Background()

	t.Run("successful delete index", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		// Setup existing index
		mockKV.data["index-key"] = []byte{}

		err := repo.DeleteIndex(ctx, "index-key")

		assert.NoError(t, err)

		// Verify index was deleted
		_, exists := mockKV.data["index-key"]
		assert.False(t, exists)
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		err := repo.DeleteIndex(ctx, "index-key")

		assert.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})
}

func TestNatsBaseRepository_ListKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list keys", func(t *testing.T) {
		mockKV := newMockNatsKeyValue()
		repo := NewNatsBaseRepository[TestEntity](mockKV, "test")

		mockKV.data["key1"] = []byte("data1")
		mockKV.data["key2"] = []byte("data2")

		keys, err := repo.ListKeys(ctx)

		assert.NoError(t, err)
		assert.Len(t, keys, 2)
		assert.Contains(t, keys, "key1")
		assert.Contains(t, keys, "key2")
	})

	t.Run("repository not ready", func(t *testing.T) {
		repo := NewNatsBaseRepository[TestEntity](nil, "test")

		keys, err := repo.ListKeys(ctx)

		assert.Error(t, err)
		assert.Nil(t, keys)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})
}
