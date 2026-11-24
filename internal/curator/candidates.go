package curator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"

	"github.com/memtwig/memtwig/internal/embed"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/vault"
)

const (
	defaultMaxCandidatePairs = 6
	defaultMinCosine         = 0.55
	maxExcerptRunes          = 1400
	chunksPerExcerpt         = 3
)

// CandidatePair describes a pair of related notes surfaced for contradiction/duplication review.
type CandidatePair struct {
	Vault    string  `json:"vault"`
	NoteA    string  `json:"note_a"`
	NoteB    string  `json:"note_b"`
	Score    float64 `json:"score"`
	Method   string  `json:"method"`
	ExcerptA string  `json:"excerpt_a"`
	ExcerptB string  `json:"excerpt_b"`
	HashA    string  `json:"hash_a,omitempty"`
	HashB    string  `json:"hash_b,omitempty"`
}

// CandidateOptions configures candidate note pair selection and rotation.
type CandidateOptions struct {
	DBPath       string
	MaxPairs     int
	MinCosine    float64
	IgnoreJudged bool
}

// JudgedEntry tracks when a note pair was reviewed and the note SHA256 hashes at that time.
type JudgedEntry struct {
	JudgedAt string `json:"judged_at"`
	HashA    string `json:"hash_a"`
	HashB    string `json:"hash_b"`
}

type chunkVec struct {
	text string
	vec  []float32
}

func normalizeRelPath(p string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/"))
}

func canonicalPair(noteA, hashA, noteB, hashB string) (key, ca, ha, cb, hb string) {
	ca, ha = normalizeRelPath(noteA), hashA
	cb, hb = normalizeRelPath(noteB), hashB
	if ca > cb {
		ca, cb = cb, ca
		ha, hb = hb, ha
	}
	return ca + "|" + cb, ca, ha, cb, hb
}

func judgedPairsPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".curator", "judged_pairs.json")
}

// LoadJudgedPairs reads <vaultRoot>/.curator/judged_pairs.json, returning an empty map if absent or invalid.
func LoadJudgedPairs(vaultRoot string) map[string]JudgedEntry {
	out := map[string]JudgedEntry{}
	raw, err := os.ReadFile(judgedPairsPath(vaultRoot))
	if err != nil {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]JudgedEntry{}
	}
	return out
}

// SaveJudgedPairs writes the judged-pairs state atomically under <vaultRoot>/.curator/judged_pairs.json.
func SaveJudgedPairs(vaultRoot string, m map[string]JudgedEntry) error {
	if m == nil {
		m = map[string]JudgedEntry{}
	}
	p := judgedPairsPath(vaultRoot)
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, "judged_pairs.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

// MarkPairsJudged records reviewed pairs in .curator/judged_pairs.json using canonical keys and current UTC timestamp.
func MarkPairsJudged(vaultRoot string, pairs []CandidatePair) error {
	if len(pairs) == 0 {
		return nil
	}
	m := LoadJudgedPairs(vaultRoot)
	now := time.Now().UTC().Format(time.RFC3339)
	for _, pair := range pairs {
		ca := normalizeRelPath(pair.NoteA)
		cb := normalizeRelPath(pair.NoteB)
		if ca == "" || cb == "" || ca == cb {
			continue
		}
		key, _, ha, _, hb := canonicalPair(ca, pair.HashA, cb, pair.HashB)
		m[key] = JudgedEntry{
			JudgedAt: now,
			HashA:    ha,
			HashB:    hb,
		}
	}
	return SaveJudgedPairs(vaultRoot, m)
}

// MarkPairJudged resolves current SHA256 hashes for two notes on disk and marks the pair as judged.
func MarkPairJudged(vaultRoot, noteA, noteB string) error {
	ca := normalizeRelPath(noteA)
	cb := normalizeRelPath(noteB)
	if ca == "" || cb == "" || ca == cb {
		return errors.New("mark-pair requires two distinct note paths")
	}
	na, err := vault.ParseNote("", vaultRoot, ca)
	if err != nil {
		return fmt.Errorf("read %s: %w", ca, err)
	}
	nb, err := vault.ParseNote("", vaultRoot, cb)
	if err != nil {
		return fmt.Errorf("read %s: %w", cb, err)
	}
	return MarkPairsJudged(vaultRoot, []CandidatePair{{
		NoteA: na.RelPath,
		HashA: na.SHA256,
		NoteB: nb.RelPath,
		HashB: nb.SHA256,
	}})
}

func isPairFreshlyJudged(judged map[string]JudgedEntry, noteA, hashA, noteB, hashB string) bool {
	if len(judged) == 0 {
		return false
	}
	key, _, ha, _, hb := canonicalPair(noteA, hashA, noteB, hashB)
	e, ok := judged[key]
	if !ok {
		return false
	}
	return ha != "" && hb != "" && e.HashA == ha && e.HashB == hb
}

func isCuratorCandidateExcluded(n vault.Note) bool {
	st := strings.ToLower(strings.TrimSpace(n.Status))
	if st == "closed" || st == "chiuso" {
		return true
	}
	rel := normalizeRelPath(n.RelPath)
	if vault.IsDecisionLog(rel) {
		return true
	}
	wrapped := "/" + rel
	if strings.Contains(wrapped, "/archive/") || strings.Contains(wrapped, "/archivio/") {
		return true
	}
	switch rel {
	case "index.md", "criteri-conoscenza.md", "todo/curatore.md", "todo/curator.md":
		return true
	}
	return false
}

// FindCandidatePairs surfaces pairs of distinct notes likely covering overlapping/conflicting facts.
// Uses vector centroids + chunk cosine similarity from SQLite when available, falling back to
// same-directory keyword overlap only when fewer than 2 eligible notes have embeddings.
func FindCandidatePairs(vaultName, vaultRoot string, opts CandidateOptions) ([]CandidatePair, error) {
	if opts.MaxPairs <= 0 {
		opts.MaxPairs = defaultMaxCandidatePairs
	}
	if opts.MinCosine <= 0 {
		opts.MinCosine = defaultMinCosine
	}

	notes, err := vault.WalkNotes(vaultName, vaultRoot)
	if err != nil {
		return nil, err
	}

	eligible := make([]vault.Note, 0, len(notes))
	noteByPath := make(map[string]vault.Note, len(notes))
	for _, n := range notes {
		if isCuratorCandidateExcluded(n) {
			continue
		}
		eligible = append(eligible, n)
		noteByPath[n.RelPath] = n
	}
	if len(eligible) < 2 {
		return nil, nil
	}

	var judged map[string]JudgedEntry
	if !opts.IgnoreJudged {
		judged = LoadJudgedPairs(vaultRoot)
	}

	if opts.DBPath != "" {
		if _, err := os.Stat(opts.DBPath); err == nil {
			pairs, usedVector, err := findVectorCandidatePairs(vaultName, opts.DBPath, noteByPath, judged, opts)
			if err == nil && usedVector {
				return pairs, nil
			}
		}
	}

	// Keyword fallback when SQLite embeddings are unavailable.
	var out []CandidatePair
	for i := 0; i < len(eligible) && len(out) < opts.MaxPairs; i++ {
		for j := i + 1; j < len(eligible) && len(out) < opts.MaxPairs; j++ {
			a, b := eligible[i], eligible[j]
			if !opts.IgnoreJudged && isPairFreshlyJudged(judged, a.RelPath, a.SHA256, b.RelPath, b.SHA256) {
				continue
			}
			if shareTopDir(a.RelPath, b.RelPath) && shareKeywords(a.Title+" "+a.RelPath, b.Title+" "+b.RelPath) {
				out = append(out, CandidatePair{
					Vault:    vaultName,
					NoteA:    a.RelPath,
					NoteB:    b.RelPath,
					Score:    0,
					Method:   "keyword",
					ExcerptA: trimRunes(a.Body, maxExcerptRunes),
					ExcerptB: trimRunes(b.Body, maxExcerptRunes),
					HashA:    a.SHA256,
					HashB:    b.SHA256,
				})
			}
		}
	}
	return out, nil
}

func findVectorCandidatePairs(vaultName, dbPath string, noteByPath map[string]vault.Note, judged map[string]JudgedEntry, opts CandidateOptions) ([]CandidatePair, bool, error) {
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, false, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT rel_path, heading, content, embedding FROM chunks WHERE vault_id = ? AND embedding IS NOT NULL ORDER BY id`,
		vaultName,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	perNote := map[string][]chunkVec{}
	expectedDim := 0
	for rows.Next() {
		var relPath, heading, content string
		var blob []byte
		if err := rows.Scan(&relPath, &heading, &content, &blob); err != nil {
			continue
		}
		relPath = normalizeRelPath(relPath)
		if _, ok := noteByPath[relPath]; !ok {
			continue
		}
		vec := index.DecodeFloat32s(blob)
		if len(vec) == 0 {
			continue
		}
		if expectedDim == 0 {
			expectedDim = len(vec)
		} else if len(vec) != expectedDim {
			continue
		}
		perNote[relPath] = append(perNote[relPath], chunkVec{
			text: formatChunkExcerptText(heading, content),
			vec:  embed.Normalize(vec),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(perNote) < 2 {
		return nil, false, nil
	}

	paths := make([]string, 0, len(perNote))
	centroids := make(map[string][]float32, len(perNote))
	for p, chunks := range perNote {
		paths = append(paths, p)
		acc := make([]float64, expectedDim)
		for _, ch := range chunks {
			for k, v := range ch.vec {
				acc[k] += float64(v)
			}
		}
		sumVec := make([]float32, expectedDim)
		for k, v := range acc {
			sumVec[k] = float32(v)
		}
		centroids[p] = embed.Normalize(sumVec)
	}
	sort.Strings(paths)

	type scoredNotePair struct {
		cos float64
		a   string
		b   string
	}
	var notePairs []scoredNotePair
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			a, b := paths[i], paths[j]
			na, nb := noteByPath[a], noteByPath[b]
			if !opts.IgnoreJudged && isPairFreshlyJudged(judged, a, na.SHA256, b, nb.SHA256) {
				continue
			}
			c := embed.Cosine(centroids[a], centroids[b])
			if c >= opts.MinCosine {
				notePairs = append(notePairs, scoredNotePair{cos: c, a: a, b: b})
			}
		}
	}
	sort.Slice(notePairs, func(i, j int) bool {
		if notePairs[i].cos != notePairs[j].cos {
			return notePairs[i].cos > notePairs[j].cos
		}
		if notePairs[i].a != notePairs[j].a {
			return notePairs[i].a < notePairs[j].a
		}
		return notePairs[i].b < notePairs[j].b
	})

	out := make([]CandidatePair, 0, min(opts.MaxPairs, len(notePairs)))
	for _, np := range notePairs {
		if len(out) >= opts.MaxPairs {
			break
		}
		chunksA := perNote[np.a]
		chunksB := perNote[np.b]
		bestCos := -1.0
		bestIA, bestIB := 0, 0
		for ia, ca := range chunksA {
			for ib, cb := range chunksB {
				c := embed.Cosine(ca.vec, cb.vec)
				if c > bestCos {
					bestCos = c
					bestIA, bestIB = ia, ib
				}
			}
		}
		if bestCos < opts.MinCosine {
			continue
		}
		na, nb := noteByPath[np.a], noteByPath[np.b]
		out = append(out, CandidatePair{
			Vault:    vaultName,
			NoteA:    np.a,
			NoteB:    np.b,
			Score:    math.Round(bestCos*10000) / 10000,
			Method:   "vector",
			ExcerptA: buildWindowExcerpt(chunksA, bestIA),
			ExcerptB: buildWindowExcerpt(chunksB, bestIB),
			HashA:    na.SHA256,
			HashB:    nb.SHA256,
		})
	}
	return out, true, nil
}

func formatChunkExcerptText(heading, content string) string {
	c := strings.TrimSpace(content)
	h := strings.TrimSpace(heading)
	if h == "" || strings.HasPrefix(c, "#") {
		return c
	}
	return "[" + h + "] " + c
}

func buildWindowExcerpt(chunks []chunkVec, start int) string {
	end := min(len(chunks), start+chunksPerExcerpt)
	parts := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if chunks[i].text != "" {
			parts = append(parts, chunks[i].text)
		}
	}
	return trimRunes(strings.Join(parts, "\n"), maxExcerptRunes)
}

func trimRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return strings.TrimSpace(s[:i])
		}
		n++
	}
	return s
}

func shareTopDir(a, b string) bool {
	pa := strings.Split(a, "/")[0]
	pb := strings.Split(b, "/")[0]
	return pa != "" && pa == pb
}

func shareKeywords(a, b string) bool {
	repl := strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ")
	wordsA := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(repl.Replace(a))) {
		if len(w) >= 4 {
			wordsA[w] = true
		}
	}
	for _, w := range strings.Fields(strings.ToLower(repl.Replace(b))) {
		if len(w) >= 4 && wordsA[w] {
			return true
		}
	}
	return false
}
