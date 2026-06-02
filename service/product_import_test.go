package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const sampleCSV = `"Code article","Collection","Rayon","Famille","Taille","Couleur","Libellé article","Prix Détail (TTC)"," physique"
"CE26-MCH13","","","","","","","",""
"","E26","HOMME","CHEMISE","M","VERT","CHEMISE MEN ETE","89.99","10"
"","E26","HOMME","CHEMISE","M","MARRON","CHEMISE MEN ETE","89.99","6"
"","E26","HOMME","CHEMISE","L","VERT","CHEMISE MEN ETE","89.99","4"
"","","","","","","","",""
"CE26-MPU01","","","","","","","",""
"","E26","HOMME","PULL","M","BLANC","PULL MEN ""ME""","69.99","10"
"","E26","HOMME","PULL","M","BLANC/NOIR","PULL MEN ""ME""","69.99","3"
"","","","","","","","",""`

func TestParseProductsCSV(t *testing.T) {
	products, err := parseProductsCSV(strings.NewReader(sampleCSV))
	assert.NoError(t, err)
	assert.Len(t, products, 2)

	chemise := products[0]
	assert.Equal(t, "CE26-MCH13", chemise.SKU)
	assert.Equal(t, "E26", chemise.Collection)
	assert.Equal(t, "HOMME", chemise.Gender)
	assert.Equal(t, "CHEMISE", chemise.Famille)
	assert.Equal(t, "CHEMISE MEN ETE", chemise.Name)
	assert.Equal(t, 89.99, chemise.Price)
	assert.Len(t, chemise.Variants, 3)

	pull := products[1]
	assert.Equal(t, "CE26-MPU01", pull.SKU)
	assert.Equal(t, `PULL MEN "ME"`, pull.Name) // les guillemets échappés sont conservés
	assert.Len(t, pull.Variants, 2)
}

func TestBuildProductDefaults(t *testing.T) {
	products, err := parseProductsCSV(strings.NewReader(sampleCSV))
	assert.NoError(t, err)

	imp := &ProductImporter{}
	used := map[string]bool{}

	product, _, sizes, colors, variants := imp.buildProduct(products[0], nil, used)

	assert.Equal(t, "homme", product.Gender)                  // HOMME -> homme
	assert.Equal(t, defaultCurrency, product.Currency)        // TND par défaut
	assert.True(t, product.IsActive)                          // actif par défaut
	assert.NotEmpty(t, product.Slug)                          // slug obligatoire généré
	assert.NotNil(t, product.SKU)
	assert.Equal(t, "CE26-MCH13", *product.SKU)
	assert.Equal(t, []string{"E26"}, []string(product.Tags))

	// 2 tailles distinctes (M, L), 2 couleurs distinctes (VERT, MARRON)
	assert.Len(t, sizes, 2)
	assert.Len(t, colors, 2)
	assert.Len(t, variants, 3)

	// Hex obligatoire toujours renseigné.
	for _, v := range variants {
		assert.NotEmpty(t, v.Hex)
	}
	// Stock agrégé pour la taille M (10 + 6 = 16).
	assert.Equal(t, 16, sizes[0].Stock)
}

func TestHexForColor(t *testing.T) {
	assert.Equal(t, "#000000", hexForColor("NOIR"))
	assert.Equal(t, "#FFFFFF", hexForColor("blanc"))
	assert.Equal(t, "#FFFFFF", hexForColor("BLANC/NOIR")) // couleur composée -> 1ère teinte
	assert.Equal(t, defaultHex, hexForColor(""))          // repli
	assert.Equal(t, defaultHex, hexForColor("TURQUOISE")) // inconnue -> repli
}

func TestNormalizeGender(t *testing.T) {
	assert.Equal(t, "homme", normalizeGender("HOMME"))
	assert.Equal(t, "femme", normalizeGender("Femme"))
	assert.Equal(t, "homme", normalizeGender("")) // repli homme
}

func TestParsePriceAndStock(t *testing.T) {
	assert.Equal(t, 89.99, parsePrice("89.99"))
	assert.Equal(t, 89.99, parsePrice("89,99")) // virgule tolérée
	assert.Equal(t, 0.0, parsePrice(""))
	assert.Equal(t, 0.0, parsePrice("N/A"))

	assert.Equal(t, 10, parseStock("10"))
	assert.Equal(t, 0, parseStock(""))
	assert.Equal(t, 0, parseStock("x"))
}
