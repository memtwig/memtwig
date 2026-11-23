package search

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/embed"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/scope"
)

type Result struct {
	Vault   string  `json:"vault"`
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Heading string  `json:"heading"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Cosine  float64 `json:"cosine,omitempty"`
}

type Options struct {
	TopK        int
	NoVectors   bool
	MaxChars    int
	RequireGate bool
}

var (
	reWord = regexp.MustCompile(`[\p{L}\p{N}_-]+`)

	courtesyRe = regexp.MustCompile(`(?i)^(?:(?:grazie(?:\s+mille|\s+tanto)?|ciao|ok|okay|va\s+bene|vai|s[iì]|no|certo|perfetto|esatto|capito|ricevuto|buonanotte|buongiorno|buonasera|quindi|dai|bene|top|ottimo|procedi|continua|fatto|pure|allora|a\s+dopo|hey|hi|hello|yo|thanks?(?:\s+a\s+lot)?|ty|thx|yes|yep|yup|yeah|nope|nah|k|kk|sure|cool|great|nice|awesome|sweet|gotcha|all\s+good|sounds?\s+good|go\s+ahead|continue|proceed|got\s+it|done|alright|later|nvm|never\s*mind)[\s,.!?;:"'\x{2026}]*)+$`)

	correctionRe = regexp.MustCompile(`(?i)(?:^\s*no[,;:.!]|\bsmettila\b|\bti\s+(?:avevo|ho)\s+(?:detto|chiesto)\b|\bla\s+prossima\s+volta\b|\bhai\s+(?:sbagliato|dimenticato|fatto\s+male)\b|\bnon\s+va\s+bene\b|\bnon\s+(?:e|è)\s+(?:cos[iì]|vero)\b|\bstop\s+(?:doing|using|writing|adding)\b|\bnext\s+time\b|\bi\s+(?:already\s+)?told\s+you\b|\byou\s+(?:forgot|missed|got\s+that\s+wrong)\b|\bdon['’]?t\s+(?:do|use|write|add)\s+(?:that|this|it)\s+anymore\b)`)

	stopwords = map[string]bool{
		// Italian
		"che": true, "chi": true, "cui": true, "con": true, "per": true, "del": true, "della": true,
		"dello": true, "dei": true, "degli": true, "delle": true, "dal": true, "dalla": true, "dalle": true,
		"nel": true, "nella": true, "nelle": true, "sul": true, "sulla": true, "sulle": true, "una": true,
		"uno": true, "gli": true, "come": true, "quando": true, "dove": true, "cosa": true, "quale": true,
		"quali": true, "quanto": true, "perche": true, "perché": true, "sono": true, "essere": true,
		"avere": true, "anche": true, "piu": true, "più": true, "ancora": true, "adesso": true, "allora": true,
		"questo": true, "questa": true, "questi": true, "queste": true, "quello": true, "quella": true,
		"non": true, "devo": true, "devi": true, "fammi": true, "dimmi": true, "spiega": true,
		// English
		"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true,
		"have": true, "has": true, "are": true, "was": true, "were": true, "what": true, "when": true,
		"where": true, "which": true, "who": true, "how": true, "can": true, "could": true, "would": true,
		"should": true, "please": true, "about": true, "into": true, "your": true, "our": true,
	}
)

// IsCourtesy returns true if the prompt is pure filler/courtesy that should not trigger memory injection.
func IsCourtesy(prompt string) bool {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return true
	}
	return courtesyRe.MatchString(p)
}

// IsCorrection returns true if the user message looks like a correction to persist.
func IsCorrection(prompt string) bool {
	return correctionRe.MatchString(prompt)
}

// Query runs hybrid BM25 + optional vector search fused with RRF, strictly scoped to activeVaults.
func Query(ctx context.Context, st *index.Store, cfg *config.Config, activeVaults []scope.ActiveVault, query string, opts Options) ([]Result, string, error) {
	if len(activeVaults) == 0 {
		return nil, "", nil
	}
	if opts.TopK <= 0 {
		opts.TopK = cfg.Retrieval.DefaultTopK
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = cfg.Retrieval.MaxSnippetChars
	}
	rrfK := cfg.Retrieval.RRFK
	if rrfK <= 0 {
		rrfK = 60
	}

	if opts.RequireGate && IsCourtesy(query) {
		return nil, "", nil
	}

	vaultMap := map[string]scope.ActiveVault{}
	placeholders := make([]string, 0, len(activeVaults))
	vaultArgs := make([]any, 0, len(activeVaults))
	for _, av := range activeVaults {
		vaultMap[av.Name] = av
		placeholders = append(placeholders, "?")
		vaultArgs = append(vaultArgs, av.Name)
	}
	inClause := strings.Join(placeholders, ",")

	// Extract meaningful terms
	rawWords := reWord.FindAllString(strings.ToLower(query), -1)
	var terms []string
	seenTerm := map[string]bool{}
	for _, w := range rawWords {
		if len([]rune(w)) < 2 || stopwords[w] || seenTerm[w] {
			continue
		}
		seenTerm[w] = true
		terms = append(terms, w)
	}

	if opts.RequireGate && len(terms) == 0 {
		return nil, "", nil
	}

	// 1. BM25 branch (scoped in SQL)
	bmRank := map[int64]int{}
	chunkCache := map[int64]index.Chunk{}
	if len(terms) > 0 {
		quoted := make([]string, len(terms))
		for i, t := range terms {
			clean := strings.ReplaceAll(t, `"`, ``)
			if len([]rune(clean)) >= 3 {
				quoted[i] = `"` + clean + `"*`
			} else {
				quoted[i] = `"` + clean + `"`
			}
		}
		ftsExpr := strings.Join(quoted, " OR ")
		sqlQ := fmt.Sprintf(`
			SELECT c.id, c.vault_id, c.rel_path, c.title, c.heading, c.content
			FROM chunks_fts f
			JOIN chunks c ON c.id = f.rowid
			WHERE chunks_fts MATCH ? AND c.vault_id IN (%s)
			ORDER BY bm25(chunks_fts)
			LIMIT 40
		`, inClause)
		args := append([]any{ftsExpr}, vaultArgs...)
		rows, err := st.DB.QueryContext(ctx, sqlQ, args...)
		if err == nil {
			rank := 1
			for rows.Next() {
				var c index.Chunk
				if err := rows.Scan(&c.ID, &c.VaultID, &c.RelPath, &c.Title, &c.Heading, &c.Content); err == nil {
					if av, ok := vaultMap[c.VaultID]; ok && av.AllowsPath(c.RelPath) {
						bmRank[c.ID] = rank
						chunkCache[c.ID] = c
						rank++
					}
				}
			}
			rows.Close()
		}
	}

	// 2. Vector branch (optional, graceful degradation)
	vecRank := map[int64]int{}
	vecCos := map[int64]float64{}
	var warn string
	prov := embed.New(cfg.Embeddings)
	if !opts.NoVectors && prov.Name() != "none" {
		qVecs, err := prov.Embed(ctx, []string{query})
		if err != nil {
			warn = fmt.Sprintf("embeddings unavailable (%v), falling back to BM25", err)
		} else if len(qVecs) == 1 && len(qVecs[0]) > 0 {
			qv := qVecs[0]
			sqlV := fmt.Sprintf(`
				SELECT id, vault_id, rel_path, title, heading, content, embedding
				FROM chunks
				WHERE vault_id IN (%s) AND embedding IS NOT NULL AND length(embedding) > 0
			`, inClause)
			rows, err := st.DB.QueryContext(ctx, sqlV, vaultArgs...)
			if err == nil {
				type scored struct {
					chunk index.Chunk
					cos   float64
				}
				var hits []scored
				for rows.Next() {
					var c index.Chunk
					var blob []byte
					if err := rows.Scan(&c.ID, &c.VaultID, &c.RelPath, &c.Title, &c.Heading, &c.Content, &blob); err == nil {
						if av, ok := vaultMap[c.VaultID]; !ok || !av.AllowsPath(c.RelPath) {
							continue
						}
						v := index.DecodeFloat32s(blob)
						cos := embed.Cosine(qv, v)
						if cos >= cfg.Embeddings.CosineThreshold {
							hits = append(hits, scored{chunk: c, cos: cos})
						}
					}
				}
				rows.Close()
				sort.Slice(hits, func(i, j int) bool { return hits[i].cos > hits[j].cos })
				if len(hits) > 40 {
					hits = hits[:40]
				}
				for i, h := range hits {
					vecRank[h.chunk.ID] = i + 1
					vecCos[h.chunk.ID] = h.cos
					if _, exists := chunkCache[h.chunk.ID]; !exists {
						chunkCache[h.chunk.ID] = h.chunk
					}
				}
			}
		}
	}

	// 3. RRF Fusion + deduplicate by (vault, rel_path) keeping best chunk
	type docAgg struct {
		chunk  index.Chunk
		score  float64
		cosine float64
	}
	byDoc := map[string]*docAgg{}
	for id, c := range chunkCache {
		var score float64
		if r, ok := bmRank[id]; ok {
			score += 1.0 / float64(rrfK+r)
		}
		if r, ok := vecRank[id]; ok {
			score += 1.0 / float64(rrfK+r)
		}
		key := c.VaultID + ":" + c.RelPath
		if prev, ok := byDoc[key]; !ok || score > prev.score {
			byDoc[key] = &docAgg{chunk: c, score: score, cosine: vecCos[id]}
		}
	}

	out := make([]Result, 0, len(byDoc))
	for _, a := range byDoc {
		out = append(out, Result{
			Vault:   a.chunk.VaultID,
			Path:    a.chunk.RelPath,
			Title:   a.chunk.Title,
			Heading: a.chunk.Heading,
			Snippet: makeSnippet(a.chunk.Content, opts.MaxChars),
			Score:   a.score,
			Cosine:  a.cosine,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > opts.TopK {
		out = out[:opts.TopK]
	}
	return out, warn, nil
}

func makeSnippet(s string, maxChars int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "…"
}
