package memory

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

const vectorFileMagic = "NVMV1"

type vectorMatch struct {
	ID    string
	Score float64
}

func encodeVectors(vectors map[string][]float32) ([]byte, error) {
	ids := make([]string, 0, len(vectors))
	for id, vector := range vectors {
		if len(vector) > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var buffer bytes.Buffer
	buffer.WriteString(vectorFileMagic)
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(ids))); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if len(id) > math.MaxUint16 {
			return nil, fmt.Errorf("vector id is too long: %d bytes", len(id))
		}
		vector := vectors[id]
		if err := binary.Write(&buffer, binary.LittleEndian, uint16(len(id))); err != nil {
			return nil, err
		}
		buffer.WriteString(id)
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(vector))); err != nil {
			return nil, err
		}
		for _, value := range vector {
			if err := binary.Write(&buffer, binary.LittleEndian, math.Float32bits(value)); err != nil {
				return nil, err
			}
		}
	}
	return buffer.Bytes(), nil
}

func decodeVectors(data []byte) (map[string][]float32, error) {
	if len(data) < len(vectorFileMagic)+4 || string(data[:len(vectorFileMagic)]) != vectorFileMagic {
		return nil, fmt.Errorf("invalid narrative memory vector file")
	}
	reader := bytes.NewReader(data[len(vectorFileMagic):])
	var count uint32
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return nil, err
	}
	vectors := make(map[string][]float32, count)
	for i := uint32(0); i < count; i++ {
		var idLength uint16
		if err := binary.Read(reader, binary.LittleEndian, &idLength); err != nil {
			return nil, err
		}
		idBytes := make([]byte, idLength)
		if _, err := io.ReadFull(reader, idBytes); err != nil {
			return nil, err
		}
		var dimensions uint32
		if err := binary.Read(reader, binary.LittleEndian, &dimensions); err != nil {
			return nil, err
		}
		if dimensions == 0 || dimensions > 1_000_000 {
			return nil, fmt.Errorf("invalid vector dimensions %d for %q", dimensions, string(idBytes))
		}
		vector := make([]float32, dimensions)
		for j := range vector {
			var bits uint32
			if err := binary.Read(reader, binary.LittleEndian, &bits); err != nil {
				return nil, err
			}
			vector[j] = math.Float32frombits(bits)
		}
		vectors[string(idBytes)] = vector
	}
	if reader.Len() != 0 {
		return nil, fmt.Errorf("narrative memory vector file has %d trailing bytes", reader.Len())
	}
	return vectors, nil
}

func vectorFileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func searchVectors(query []float32, vectors map[string][]float32, allowed map[string]bool, limit int) []vectorMatch {
	if len(query) == 0 || len(vectors) == 0 {
		return nil
	}
	hits := make([]vectorMatch, 0)
	for id, vector := range vectors {
		if allowed != nil && !allowed[id] {
			continue
		}
		score := cosine(query, vector)
		if score > 0 {
			hits = append(hits, vectorMatch{ID: id, Score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func cosine(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / math.Sqrt(leftNorm*rightNorm)
}
