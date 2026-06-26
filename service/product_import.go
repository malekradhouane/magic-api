package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/text"
)

// ============================================================================
// Import de produits depuis un fichier CSV (export "Rayon", ex: E26-HOMME.csv)
// ============================================================================
//
// Structure du fichier source :
//
//	"Code article","Collection","Rayon","Famille","Taille","Couleur","Libellé article","Prix Détail (TTC)"," physique"
//	"CE26-MCH13","","","","","","","",""                         <- ligne d'en-tête produit (porte le SKU)
//	"","E26","HOMME","CHEMISE","M","VERT","CHEMISE MEN ETE","89.99","10"   <- ligne variante
//	...
//	"","","","","","","","",""                                   <- séparateur entre produits
//
// Un produit regroupe donc l'ensemble des lignes variantes situées entre sa
// ligne d'en-tête (SKU) et le prochain SKU / séparateur. Chaque ligne variante
// correspond à un couple (Taille, Couleur) avec son stock physique.

// Indices de colonnes du CSV source.
const (
	colCodeArticle = 0 // "Code article"      -> SKU (porté par la ligne d'en-tête)
	colCollection  = 1 // "Collection"        -> tag produit (ex: E26)
	colRayon       = 2 // "Rayon"             -> genre (HOMME -> homme)
	colFamille     = 3 // "Famille"           -> catégorie (CHEMISE, PANTALON, ...)
	colTaille      = 4 // "Taille"            -> taille de la variante
	colCouleur     = 5 // "Couleur"           -> couleur de la variante
	colLibelle     = 6 // "Libellé article"   -> nom du produit
	colPrix        = 7 // "Prix Détail (TTC)" -> prix
	colPhysique    = 8 // " physique"         -> stock physique de la variante
)

// Valeurs par défaut appliquées lorsque le CSV ne porte pas une donnée
// obligatoire pour la structure Product.
const (
	defaultGender      = "homme"         // ce fichier ne concerne que les hommes
	defaultHex         = "#000000"       // couleur de repli (Hex est obligatoire)
	defaultCurrency    = "TND"           // devise par défaut du modèle
	defaultProductName = "Produit homme" // nom de repli si le libellé est vide
	defaultCategoryFam = "vetements"     // catégorie de repli pour une famille inconnue
)

// familleToCategorySlug fait correspondre la "Famille" du CSV au slug de la
// taxonomie initialisée par SeedDefaultCategories.
var familleToCategorySlug = map[string]string{
	"CHEMISE":  "chemises",
	"PANTALON": "pantalons",
	"PULL":     "pulls-gilets",
	"ENSEMBLE": "vetements",
}

// colorNameToHex associe les libellés de couleur du CSV à une valeur hex.
// Toute couleur inconnue retombe sur defaultHex.
var colorNameToHex = map[string]string{
	"NOIR":           "#000000",
	"BLANC":          "#FFFFFF",
	"OFF WHITE":      "#FAF9F6",
	"ECRU":           "#F0EAD6",
	"GREGE":          "#B7A99A",
	"BEIGE":          "#E8DCC0",
	"CAMEL":          "#C19A6B",
	"MARRON":         "#6B4423",
	"GRIS":           "#808080",
	"GRIS CLR":       "#C0C0C0",
	"GRIS CHARBON":   "#36454F",
	"BLEU":           "#1D4ED8",
	"BLEU CIEL":      "#87CEEB",
	"BLEU MARINE":    "#1B2A4A",
	"BLEU NUIT":      "#0D1B3E",
	"VERT":           "#2E7D32",
	"VERT MILITAIRE": "#4B5320",
	"KAKI":           "#78866B",
	"ROUGE":          "#DC2626",
	"ROUGE BRIKE":    "#9E3A26",
	"AUBERGINE":      "#5B2333",
}

// ProductImporter insère des produits issus d'un CSV en réutilisant le store
// produit existant. Il s'appuie sur la liste des catégories pour résoudre la
// famille -> category_id.
type ProductImporter struct {
	store      types.ProductStore
	categories *CategoryService
	logger     *logrus.Logger
}

// NewProductImporter construit un importeur de produits.
func NewProductImporter(products *ProductService, categories *CategoryService, logger *logrus.Logger) *ProductImporter {
	if logger == nil {
		logger = logrus.New()
	}
	return &ProductImporter{
		store:      products.store,
		categories: categories,
		logger:     logger,
	}
}

// ImportResult résume le déroulement d'un import.
type ImportResult struct {
	ProductsCreated int           `json:"products_created"`
	ProductsSkipped int           `json:"products_skipped"` // déjà présents (slug existant)
	ProductsFailed  int           `json:"products_failed"`
	VariantsCreated int           `json:"variants_created"`
	Errors          []ImportError `json:"errors,omitempty"`
}

// ImportError décrit l'échec d'un produit donné.
type ImportError struct {
	SKU  string `json:"sku"`
	Name string `json:"name"`
	Err  string `json:"error"`
}

// rawProduct est un produit regroupé tel que lu dans le CSV, avant mapping.
type rawProduct struct {
	SKU        string
	Collection string
	Gender     string
	Famille    string
	Name       string
	Price      float64
	Variants   []rawVariant
}

type rawVariant struct {
	Size  string
	Color string
	Stock int
}

// ImportFromFile ouvre le fichier puis délègue à ImportFromCSV.
func (imp *ProductImporter) ImportFromFile(ctx context.Context, path string) (*ImportResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("impossible d'ouvrir le fichier %q: %w", path, err)
	}
	defer f.Close()
	return imp.ImportFromCSV(ctx, f)
}

// ImportFromCSV lit le flux CSV, regroupe les lignes en produits, applique les
// valeurs par défaut pour les champs obligatoires manquants, puis insère chaque
// produit avec ses variantes / tailles / couleurs.
//
// L'import est idempotent : un produit dont le slug existe déjà est ignoré
// (ProductsSkipped) plutôt que recréé.
func (imp *ProductImporter) ImportFromCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	products, err := parseProductsCSV(r)
	if err != nil {
		return nil, err
	}

	// Résolution famille -> category_id (chargée une seule fois).
	categoryIDBySlug, err := imp.categorySlugIndex(ctx)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}
	usedSlugs := make(map[string]bool)

	for _, rp := range products {
		product, images, sizes, colors, variants := imp.buildProduct(rp, categoryIDBySlug, usedSlugs)

		// Idempotence : si le slug existe déjà en base, on saute.
		if _, err := imp.store.GetBySlug(ctx, product.Slug); err == nil {
			result.ProductsSkipped++
			imp.logger.WithField("slug", product.Slug).Info("produit déjà présent, ignoré")
			continue
		} else if !errs.IsNoSuchEntityError(err) {
			result.ProductsFailed++
			result.Errors = append(result.Errors, ImportError{SKU: rp.SKU, Name: product.Name, Err: err.Error()})
			continue
		}

		if _, err := imp.store.Create(ctx, product, images, sizes, colors, variants); err != nil {
			result.ProductsFailed++
			result.Errors = append(result.Errors, ImportError{SKU: rp.SKU, Name: product.Name, Err: err.Error()})
			imp.logger.WithError(err).WithField("sku", rp.SKU).Error("échec de création du produit")
			continue
		}

		result.ProductsCreated++
		result.VariantsCreated += len(variants)
	}

	return result, nil
}

// buildProduct transforme un rawProduct en entités persistables, en appliquant
// les valeurs par défaut requises (genre, devise, hex, nom, catégorie).
func (imp *ProductImporter) buildProduct(
	rp *rawProduct,
	categoryIDBySlug map[string]uuid.UUID,
	usedSlugs map[string]bool,
) (*interfaces.Product, []interfaces.ProductImage, []interfaces.ProductSize, []interfaces.ProductColor, []interfaces.ProductVariant) {

	name := strings.TrimSpace(rp.Name)
	if name == "" {
		name = defaultProductName
	}

	product := &interfaces.Product{
		Slug:        imp.uniqueSlug(name, rp.SKU, usedSlugs),
		Name:        name,
		Description: name,
		Price:       rp.Price, // 0 par défaut si absent / illisible
		Currency:    defaultCurrency,
		Gender:      normalizeGender(rp.Gender),
		IsActive:    true,
	}

	// SKU : pointeur nul si absent (colonne uniqueIndex, NULL accepté).
	if sku := strings.TrimSpace(rp.SKU); sku != "" {
		product.SKU = &sku
	}

	// Tag de collection (ex: E26).
	if col := strings.TrimSpace(rp.Collection); col != "" {
		product.Tags = interfaces.StringArray{col}
	}

	// Catégorie déduite de la famille (repli sur "vetements").
	if catID, ok := imp.resolveCategory(rp.Famille, categoryIDBySlug); ok {
		product.CategoryID = &catID
	}

	// Construction des variantes + agrégation tailles / couleurs (ordre préservé).
	variants := make([]interfaces.ProductVariant, 0, len(rp.Variants))

	sizeOrder := make([]string, 0)
	sizeStock := make(map[string]int)
	colorOrder := make([]string, 0)
	colorStock := make(map[string]int)

	for _, rv := range rp.Variants {
		size := strings.TrimSpace(rv.Size)
		color := strings.TrimSpace(rv.Color)
		hex := hexForColor(color)

		variants = append(variants, interfaces.ProductVariant{
			Size:     size,
			Color:    color,
			Hex:      hex,
			Stock:    rv.Stock,
			Position: len(variants),
		})

		if size != "" {
			if _, seen := sizeStock[size]; !seen {
				sizeOrder = append(sizeOrder, size)
			}
			sizeStock[size] += rv.Stock
		}
		if color != "" {
			if _, seen := colorStock[color]; !seen {
				colorOrder = append(colorOrder, color)
			}
			colorStock[color] += rv.Stock
		}
	}

	sizes := make([]interfaces.ProductSize, 0, len(sizeOrder))
	for i, s := range sizeOrder {
		sizes = append(sizes, interfaces.ProductSize{Size: s, Stock: sizeStock[s], Position: i})
	}

	colors := make([]interfaces.ProductColor, 0, len(colorOrder))
	for i, c := range colorOrder {
		colors = append(colors, interfaces.ProductColor{
			Name:     c,
			Hex:      hexForColor(c),
			Stock:    colorStock[c],
			Position: i,
		})
	}

	// Pas d'images dans ce CSV.
	images := []interfaces.ProductImage{}

	return product, images, sizes, colors, variants
}

// resolveCategory renvoie l'UUID de catégorie pour une famille donnée.
func (imp *ProductImporter) resolveCategory(famille string, idBySlug map[string]uuid.UUID) (uuid.UUID, bool) {
	slug, ok := familleToCategorySlug[strings.ToUpper(strings.TrimSpace(famille))]
	if !ok {
		slug = defaultCategoryFam
	}
	id, ok := idBySlug[slug]
	if !ok {
		// repli ultime sur la catégorie générique vêtements
		id, ok = idBySlug[defaultCategoryFam]
	}
	return id, ok
}

// categorySlugIndex charge la liste des catégories et indexe slug -> id.
func (imp *ProductImporter) categorySlugIndex(ctx context.Context) (map[string]uuid.UUID, error) {
	cats, err := imp.categories.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("impossible de charger les catégories: %w", err)
	}
	idx := make(map[string]uuid.UUID, len(cats))
	for _, c := range cats {
		idx[c.Slug] = c.ID
	}
	return idx, nil
}

// uniqueSlug génère un slug unique au sein de l'import (suffixe SKU puis
// compteur en cas de collision sur le nom).
func (imp *ProductImporter) uniqueSlug(name, sku string, used map[string]bool) string {
	base := text.Slugify(name)
	if base == "" {
		base = text.Slugify(sku)
	}
	if base == "" {
		base = "produit"
	}

	slug := base
	if used[slug] {
		if s := text.Slugify(sku); s != "" {
			slug = base + "-" + s
		}
	}
	for i := 2; used[slug]; i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	used[slug] = true
	return slug
}

// parseProductsCSV lit l'intégralité du CSV et regroupe les lignes en produits.
func parseProductsCSV(r io.Reader) ([]*rawProduct, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // lignes de longueur variable tolérées
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("lecture CSV impossible: %w", err)
	}

	var (
		products []*rawProduct
		current  *rawProduct
	)

	field := func(rec []string, idx int) string {
		if idx < len(rec) {
			return strings.TrimSpace(rec[idx])
		}
		return ""
	}

	for _, rec := range records {
		code := field(rec, colCodeArticle)
		famille := field(rec, colFamille)
		taille := field(rec, colTaille)
		couleur := field(rec, colCouleur)

		// Ligne d'en-tête du fichier.
		if strings.EqualFold(code, "Code article") {
			continue
		}
		// Ligne entièrement vide (séparateur).
		if isEmptyRecord(rec) {
			continue
		}

		// Ligne d'en-tête produit : ne porte que le SKU.
		if code != "" && famille == "" && taille == "" && couleur == "" {
			current = &rawProduct{SKU: code}
			products = append(products, current)
			continue
		}

		// Ligne variante (au moins une donnée article présente).
		if famille != "" || taille != "" || couleur != "" {
			if current == nil {
				current = &rawProduct{}
				products = append(products, current)
			}
			if current.Famille == "" {
				current.Famille = famille
			}
			if current.Collection == "" {
				current.Collection = field(rec, colCollection)
			}
			if current.Gender == "" {
				current.Gender = field(rec, colRayon)
			}
			if current.Name == "" {
				current.Name = field(rec, colLibelle)
			}
			if current.Price == 0 {
				current.Price = parsePrice(field(rec, colPrix))
			}
			current.Variants = append(current.Variants, rawVariant{
				Size:  taille,
				Color: couleur,
				Stock: parseStock(field(rec, colPhysique)),
			})
		}
	}

	// On écarte les produits sans aucune variante exploitable.
	cleaned := products[:0]
	for _, p := range products {
		if len(p.Variants) > 0 {
			cleaned = append(cleaned, p)
		}
	}
	return cleaned, nil
}

// isEmptyRecord indique si toutes les colonnes d'une ligne sont vides.
func isEmptyRecord(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// normalizeGender ramène le "Rayon" du CSV sur la valeur attendue par le modèle.
func normalizeGender(rayon string) string {
	switch strings.ToUpper(strings.TrimSpace(rayon)) {
	case "HOMME":
		return "homme"
	case "FEMME":
		return "femme"
	case "ENFANT":
		return "enfant"
	case "UNISEXE", "MIXTE":
		return "unisexe"
	case "":
		return defaultGender
	default:
		return strings.ToLower(strings.TrimSpace(rayon))
	}
}

// hexForColor renvoie le hex associé à une couleur (repli defaultHex). Les
// couleurs composées (ex: "BLANC/NOIR") utilisent la première teinte connue.
func hexForColor(color string) string {
	key := strings.ToUpper(strings.TrimSpace(color))
	if key == "" {
		return defaultHex
	}
	if hex, ok := colorNameToHex[key]; ok {
		return hex
	}
	if idx := strings.IndexAny(key, "/-"); idx > 0 {
		if hex, ok := colorNameToHex[strings.TrimSpace(key[:idx])]; ok {
			return hex
		}
	}
	return defaultHex
}

// parsePrice convertit la colonne prix en float (0 si illisible).
func parsePrice(raw string) float64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseStock convertit la colonne stock physique en entier (0 si illisible).
func parseStock(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}
