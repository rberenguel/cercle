package search

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"cercle-v2/internal/indexer"
	"cercle-v2/internal/models"
)

// getShardPath returns the global path to the shard file for a given workspace and relative file path
func getShardPath(workspacePath, relFilePath string) (string, error) {
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dirHashBytes := md5.Sum([]byte(absWorkspace))
	dirHash := hex.EncodeToString(dirHashBytes[:])

	shardID := indexer.GetShardID(relFilePath)

	return filepath.Join(home, ".cercle", "indexes", dirHash, shardID+".json"), nil
}

// loadChunksForFile loads only the chunks belonging to a specific file from its corresponding shard
func loadChunksForFile(workspacePath, relFilePath string) ([]models.Chunk, error) {
	shardPath, err := getShardPath(workspacePath, relFilePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(shardPath)
	if err != nil {
		return nil, err // File might not exist if shard is empty
	}

	var allChunks []models.Chunk
	if err := json.Unmarshal(data, &allChunks); err != nil {
		return nil, err
	}

	var fileChunks []models.Chunk
	for _, chunk := range allChunks {
		if chunk.File == relFilePath {
			fileChunks = append(fileChunks, chunk)
		}
	}
	return fileChunks, nil
}

// loadAllChunks loads all chunks from all existing shards for the workspace
func loadAllChunks(workspacePath string) ([]models.Chunk, error) {
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dirHashBytes := md5.Sum([]byte(absWorkspace))
	dirHash := hex.EncodeToString(dirHashBytes[:])

	indexPath := filepath.Join(home, ".cercle", "indexes", dirHash)

	files, err := os.ReadDir(indexPath)
	if err != nil {
		return nil, err
	}

	var allChunks []models.Chunk
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" && f.Name() != "metadata.json" {
			data, err := os.ReadFile(filepath.Join(indexPath, f.Name()))
			if err != nil {
				continue
			}
			var chunks []models.Chunk
			if err := json.Unmarshal(data, &chunks); err == nil {
				allChunks = append(allChunks, chunks...)
			}
		}
	}

	return allChunks, nil
}
