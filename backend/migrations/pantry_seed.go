package migrations

import "github.com/marianamohr/financas/internal/models"

// seedRow is a compact form of PantrySeedProduct used to keep the seed readable.
// aliases are raw receipt-style names (any case/accents); they are normalized
// by the matching code, not here.
type seedRow struct {
	pt, de      string
	aliasesPt   []string
	aliasesDe   []string
	category    string
	subcategory string
	unit        string
	isProduce   bool
	shelfDays   int // 0 = unknown / not perishable
}

func intPtrOrNil(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func pantrySeedProducts() []models.PantrySeedProduct {
	rows := seedRows()
	out := make([]models.PantrySeedProduct, 0, len(rows))
	for _, r := range rows {
		aliasesPt := r.aliasesPt
		if aliasesPt == nil {
			aliasesPt = []string{}
		}
		aliasesDe := r.aliasesDe
		if aliasesDe == nil {
			aliasesDe = []string{}
		}
		out = append(out, models.PantrySeedProduct{
			NamePt:        r.pt,
			NameDe:        r.de,
			AliasesPt:     aliasesPt,
			AliasesDe:     aliasesDe,
			Category:      r.category,
			Subcategory:   r.subcategory,
			DefaultUnit:   r.unit,
			IsProduce:     r.isProduce,
			ShelfLifeDays: intPtrOrNil(r.shelfDays),
		})
	}
	return out
}

//nolint:funlen // data table
func seedRows() []seedRow {
	const (
		food     = models.PantryCategoryFood
		cleaning = models.PantryCategoryCleaning
		hygiene  = models.PantryCategoryHygiene
		general  = models.PantryCategoryGeneral
	)

	return []seedRow{
		// ── Hortifruti (frutas) ──────────────────────────────────────────────
		{pt: "Banana", de: "Banane", aliasesPt: []string{"banana prata", "banana nanica", "banana caturra", "banana kg"}, aliasesDe: []string{"bananen", "bio bananen", "banane"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 6},
		{pt: "Maçã", de: "Apfel", aliasesPt: []string{"maca", "maca fuji", "maca gala", "maca argentina"}, aliasesDe: []string{"äpfel", "apfel", "aepfel", "apfel elstar", "apfel gala"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 21},
		{pt: "Laranja", de: "Orange", aliasesPt: []string{"laranja pera", "laranja lima", "laranja bahia"}, aliasesDe: []string{"orangen", "orange"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 14},
		{pt: "Limão", de: "Zitrone", aliasesPt: []string{"limao", "limao taiti", "limao siciliano"}, aliasesDe: []string{"zitronen", "zitrone", "limetten", "limette"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 14},
		{pt: "Mamão", de: "Papaya", aliasesPt: []string{"mamao", "mamao papaya", "mamao formosa"}, aliasesDe: []string{"papaya"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Manga", de: "Mango", aliasesPt: []string{"manga palmer", "manga tommy"}, aliasesDe: []string{"mango"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Abacaxi", de: "Ananas", aliasesPt: []string{"abacaxi perola"}, aliasesDe: []string{"ananas"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Melancia", de: "Wassermelone", aliasesDe: []string{"wassermelone"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Melão", de: "Melone", aliasesPt: []string{"melao"}, aliasesDe: []string{"melone", "honigmelone", "galiamelone"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Uva", de: "Trauben", aliasesPt: []string{"uva itália", "uva thompson", "uva verde", "uva roxa"}, aliasesDe: []string{"trauben", "weintrauben", "tafeltrauben"}, category: food, subcategory: "produce", unit: "kg", isProduce: true, shelfDays: 7},
		{pt: "Morango", de: "Erdbeeren", aliasesPt: []string{"morango bandeja"}, aliasesDe: []string{"erdbeeren"}, category: food, subcategory: "produce", unit: "pct", isProduce: true, shelfDays: 3},
		{pt: "Pera", de: "Birne", aliasesPt: []string{"pera williams", "pera portuguesa"}, aliasesDe: []string{"birnen", "birne"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 10},
		{pt: "Abacate", de: "Avocado", aliasesDe: []string{"avocado", "avocados"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Kiwi", de: "Kiwi", aliasesDe: []string{"kiwi", "kiwis"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 14},
		{pt: "Tangerina", de: "Mandarine", aliasesPt: []string{"mexerica", "bergamota", "poncã"}, aliasesDe: []string{"mandarinen", "clementinen"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 10},

		// ── Hortifruti (legumes e verduras) ──────────────────────────────────
		{pt: "Tomate", de: "Tomate", aliasesPt: []string{"tomate italiano", "tomate carmem", "tomate cereja"}, aliasesDe: []string{"tomaten", "kl. tomaten", "cherrytomaten", "strauchtomaten", "rispentomaten"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Cebola", de: "Zwiebel", aliasesPt: []string{"cebola branca", "cebola roxa", "cebola kg"}, aliasesDe: []string{"zwiebeln", "zwiebeln rot", "speisezwiebeln", "gemüsezwiebel"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 30},
		{pt: "Alho", de: "Knoblauch", aliasesPt: []string{"alho cabeça", "alho kg"}, aliasesDe: []string{"knoblauch"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 60},
		{pt: "Batata", de: "Kartoffel", aliasesPt: []string{"batata inglesa", "batata kg", "batata asterix"}, aliasesDe: []string{"kartoffeln", "speisekartoffeln", "kartoffeln festkochend", "kartoffeln mehlig"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 30},
		{pt: "Batata-doce", de: "Süßkartoffel", aliasesPt: []string{"batata doce"}, aliasesDe: []string{"süßkartoffel", "suesskartoffel", "süsskartoffeln"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 21},
		{pt: "Cenoura", de: "Karotte", aliasesPt: []string{"cenoura kg"}, aliasesDe: []string{"karotten", "möhren", "moehren", "bio karotten"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 21},
		{pt: "Abobrinha", de: "Zucchini", aliasesPt: []string{"abobrinha italiana"}, aliasesDe: []string{"zucchini"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Abóbora", de: "Kürbis", aliasesPt: []string{"abobora", "abobora cabotia", "abobora moranga"}, aliasesDe: []string{"kürbis", "kuerbis", "hokkaido"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 30},
		{pt: "Pepino", de: "Gurke", aliasesPt: []string{"pepino japones"}, aliasesDe: []string{"gurke", "gurken", "salatgurke", "schlangengurke"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Pimentão", de: "Paprika", aliasesPt: []string{"pimentao", "pimentao verde", "pimentao vermelho", "pimentao amarelo"}, aliasesDe: []string{"paprika", "paprika rot", "paprika mix", "spitzpaprika"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 10},
		{pt: "Brócolis", de: "Brokkoli", aliasesPt: []string{"brocolis", "brocolis ninja"}, aliasesDe: []string{"brokkoli", "broccoli"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Couve-flor", de: "Blumenkohl", aliasesPt: []string{"couve flor"}, aliasesDe: []string{"blumenkohl"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Alface", de: "Salat", aliasesPt: []string{"alface crespa", "alface americana", "alface lisa"}, aliasesDe: []string{"salat", "kopfsalat", "eisbergsalat", "eissalat", "romana salat"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Couve", de: "Grünkohl", aliasesPt: []string{"couve manteiga", "couve maço"}, aliasesDe: []string{"grünkohl", "gruenkohl"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 4},
		{pt: "Espinafre", de: "Spinat", aliasesDe: []string{"spinat", "blattspinat"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 4},
		{pt: "Rúcula", de: "Rucola", aliasesPt: []string{"rucula"}, aliasesDe: []string{"rucola", "rauke"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 4},
		{pt: "Cheiro-verde", de: "Petersilie", aliasesPt: []string{"cheiro verde", "salsinha", "salsa", "cebolinha", "salsa e cebolinha"}, aliasesDe: []string{"petersilie", "schnittlauch"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 5},
		{pt: "Coentro", de: "Koriander", aliasesDe: []string{"koriander"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 4},
		{pt: "Manjericão", de: "Basilikum", aliasesPt: []string{"manjericao"}, aliasesDe: []string{"basilikum"}, category: food, subcategory: "produce", unit: "maco", isProduce: true, shelfDays: 5},
		{pt: "Repolho", de: "Weißkohl", aliasesPt: []string{"repolho verde", "repolho roxo"}, aliasesDe: []string{"weißkohl", "weisskohl", "rotkohl", "spitzkohl"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 14},
		{pt: "Beterraba", de: "Rote Bete", aliasesDe: []string{"rote bete", "rote beete"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 14},
		{pt: "Chuchu", de: "Chayote", aliasesDe: []string{"chayote"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 10},
		{pt: "Mandioca", de: "Maniok", aliasesPt: []string{"aipim", "macaxeira"}, aliasesDe: []string{"maniok", "cassava"}, category: food, subcategory: "produce", unit: "kg", isProduce: true, shelfDays: 5},
		{pt: "Milho verde", de: "Mais", aliasesPt: []string{"milho espiga"}, aliasesDe: []string{"mais", "maiskolben", "zuckermais"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 5},
		{pt: "Berinjela", de: "Aubergine", aliasesDe: []string{"aubergine", "auberginen"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 7},
		{pt: "Cogumelo", de: "Champignon", aliasesPt: []string{"champignon", "cogumelo paris", "shimeji", "shitake"}, aliasesDe: []string{"champignons", "pilze", "champignon braun"}, category: food, subcategory: "produce", unit: "pct", isProduce: true, shelfDays: 5},
		{pt: "Gengibre", de: "Ingwer", aliasesDe: []string{"ingwer"}, category: food, subcategory: "produce", unit: "un", isProduce: true, shelfDays: 21},

		// ── Laticínios e ovos ────────────────────────────────────────────────
		{pt: "Leite", de: "Milch", aliasesPt: []string{"leite integral", "leite desnatado", "leite semidesnatado", "leite uht", "leite longa vida"}, aliasesDe: []string{"milch", "vollmilch", "h-milch", "frische milch", "fettarme milch", "bio milch"}, category: food, subcategory: "dairy", unit: "l", shelfDays: 7},
		{pt: "Ovos", de: "Eier", aliasesPt: []string{"ovo", "ovos brancos", "ovos vermelhos", "ovos caipira", "duzia de ovos"}, aliasesDe: []string{"eier", "freilandeier", "bio eier", "bodenhaltung eier"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 21},
		{pt: "Manteiga", de: "Butter", aliasesPt: []string{"manteiga com sal", "manteiga sem sal"}, aliasesDe: []string{"butter", "deutsche markenbutter", "süßrahmbutter"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 30},
		{pt: "Margarina", de: "Margarine", aliasesDe: []string{"margarine"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 60},
		{pt: "Queijo", de: "Käse", aliasesPt: []string{"queijo mussarela", "queijo prato", "queijo minas", "queijo parmesao", "mussarela", "muçarela"}, aliasesDe: []string{"käse", "kaese", "gouda", "emmentaler", "mozzarella", "parmesan", "gratkäse"}, category: food, subcategory: "dairy", unit: "g", shelfDays: 14},
		{pt: "Requeijão", de: "Frischkäse", aliasesPt: []string{"requeijao", "requeijao cremoso"}, aliasesDe: []string{"frischkäse", "frischkaese"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 14},
		{pt: "Iogurte", de: "Joghurt", aliasesPt: []string{"iogurte natural", "iogurte grego", "iogurte morango"}, aliasesDe: []string{"joghurt", "naturjoghurt", "fruchtjoghurt", "skyr", "quark", "speisequark"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 14},
		{pt: "Creme de leite", de: "Sahne", aliasesPt: []string{"creme de leite caixinha"}, aliasesDe: []string{"sahne", "schlagsahne", "saure sahne", "schmand"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 14},
		{pt: "Leite condensado", de: "Kondensmilch", aliasesDe: []string{"kondensmilch", "gezuckerte kondensmilch"}, category: food, subcategory: "dairy", unit: "un", shelfDays: 180},

		// ── Carnes e peixes ──────────────────────────────────────────────────
		{pt: "Frango", de: "Hähnchen", aliasesPt: []string{"peito de frango", "coxa de frango", "sobrecoxa", "frango inteiro", "file de frango"}, aliasesDe: []string{"hähnchen", "haehnchen", "hähnchenbrust", "hähnchenbrustfilet", "hähnchenschenkel", "hühnerbrust"}, category: food, subcategory: "meat", unit: "kg", shelfDays: 3},
		{pt: "Carne bovina", de: "Rindfleisch", aliasesPt: []string{"carne moida", "patinho", "alcatra", "acem", "coxao mole", "picanha", "contra file", "musculo"}, aliasesDe: []string{"rindfleisch", "rinderhack", "hackfleisch", "rinderhackfleisch", "rindersteak", "gulasch"}, category: food, subcategory: "meat", unit: "kg", shelfDays: 3},
		{pt: "Carne suína", de: "Schweinefleisch", aliasesPt: []string{"carne suina", "lombo", "pernil", "costela suina", "bisteca"}, aliasesDe: []string{"schweinefleisch", "schweineschnitzel", "schweinenacken", "schweinebauch", "gemischtes hack"}, category: food, subcategory: "meat", unit: "kg", shelfDays: 3},
		{pt: "Linguiça", de: "Bratwurst", aliasesPt: []string{"linguica", "linguica toscana", "linguica calabresa", "calabresa"}, aliasesDe: []string{"bratwurst", "wiener", "würstchen", "wuerstchen"}, category: food, subcategory: "meat", unit: "kg", shelfDays: 5},
		{pt: "Presunto", de: "Schinken", aliasesPt: []string{"presunto cozido", "peito de peru", "apresuntado"}, aliasesDe: []string{"schinken", "kochschinken", "putenbrust", "aufschnitt"}, category: food, subcategory: "meat", unit: "g", shelfDays: 7},
		{pt: "Bacon", de: "Speck", aliasesDe: []string{"speck", "bacon", "frühstücksspeck"}, category: food, subcategory: "meat", unit: "g", shelfDays: 14},
		{pt: "Salame", de: "Salami", aliasesDe: []string{"salami"}, category: food, subcategory: "meat", unit: "g", shelfDays: 21},
		{pt: "Peixe", de: "Fisch", aliasesPt: []string{"file de tilapia", "tilapia", "salmao", "file de peixe", "merluza"}, aliasesDe: []string{"fisch", "lachs", "lachsfilet", "seelachs", "forelle", "fischfilet"}, category: food, subcategory: "fish", unit: "kg", shelfDays: 2},
		{pt: "Atum em lata", de: "Thunfisch", aliasesPt: []string{"atum", "atum ralado", "atum solido"}, aliasesDe: []string{"thunfisch", "thunfisch in öl", "thunfisch naturell"}, category: food, subcategory: "canned", unit: "un", shelfDays: 720},
		{pt: "Sardinha em lata", de: "Sardinen", aliasesPt: []string{"sardinha"}, aliasesDe: []string{"sardinen", "ölsardinen"}, category: food, subcategory: "canned", unit: "un", shelfDays: 720},

		// ── Grãos, massas e padaria ──────────────────────────────────────────
		{pt: "Arroz", de: "Reis", aliasesPt: []string{"arroz branco", "arroz integral", "arroz parboilizado", "arroz tipo 1"}, aliasesDe: []string{"reis", "basmati reis", "langkornreis", "jasminreis", "milchreis"}, category: food, subcategory: "grains", unit: "kg", shelfDays: 365},
		{pt: "Feijão", de: "Bohnen", aliasesPt: []string{"feijao", "feijao carioca", "feijao preto"}, aliasesDe: []string{"bohnen", "kidneybohnen", "schwarze bohnen", "weiße bohnen"}, category: food, subcategory: "grains", unit: "kg", shelfDays: 365},
		{pt: "Lentilha", de: "Linsen", aliasesDe: []string{"linsen", "rote linsen"}, category: food, subcategory: "grains", unit: "g", shelfDays: 365},
		{pt: "Grão-de-bico", de: "Kichererbsen", aliasesPt: []string{"grao de bico"}, aliasesDe: []string{"kichererbsen"}, category: food, subcategory: "grains", unit: "g", shelfDays: 365},
		{pt: "Macarrão", de: "Nudeln", aliasesPt: []string{"macarrao", "espaguete", "penne", "parafuso", "massa"}, aliasesDe: []string{"nudeln", "spaghetti", "penne", "fusilli", "pasta", "spirelli"}, category: food, subcategory: "grains", unit: "pct", shelfDays: 365},
		{pt: "Farinha de trigo", de: "Weizenmehl", aliasesPt: []string{"farinha", "farinha de trigo tipo 1"}, aliasesDe: []string{"mehl", "weizenmehl", "weizenmehl 405", "weizenmehl 550", "dinkelmehl"}, category: food, subcategory: "grains", unit: "kg", shelfDays: 365},
		{pt: "Farinha de mandioca", de: "Maniokmehl", aliasesPt: []string{"farofa", "farinha de mandioca torrada"}, aliasesDe: []string{"maniokmehl"}, category: food, subcategory: "grains", unit: "g", shelfDays: 365},
		{pt: "Fubá", de: "Maisgrieß", aliasesPt: []string{"fuba", "fuba mimoso"}, aliasesDe: []string{"maisgrieß", "polenta", "maismehl"}, category: food, subcategory: "grains", unit: "g", shelfDays: 365},
		{pt: "Aveia", de: "Haferflocken", aliasesPt: []string{"aveia em flocos"}, aliasesDe: []string{"haferflocken", "hafer"}, category: food, subcategory: "grains", unit: "g", shelfDays: 365},
		{pt: "Granola", de: "Müsli", aliasesDe: []string{"müsli", "muesli", "knuspermüsli"}, category: food, subcategory: "grains", unit: "g", shelfDays: 180},
		{pt: "Pão", de: "Brot", aliasesPt: []string{"pao", "pao frances", "pao de forma", "pao integral", "pao de leite"}, aliasesDe: []string{"brot", "brötchen", "broetchen", "toast", "toastbrot", "vollkornbrot", "roggenbrot", "körnerbrot"}, category: food, subcategory: "bakery", unit: "un", shelfDays: 4},
		{pt: "Tapioca", de: "Tapioka", aliasesPt: []string{"goma de tapioca"}, aliasesDe: []string{"tapioka"}, category: food, subcategory: "grains", unit: "g", shelfDays: 180},
		{pt: "Biscoito", de: "Kekse", aliasesPt: []string{"bolacha", "biscoito recheado", "cream cracker", "bolacha agua e sal"}, aliasesDe: []string{"kekse", "butterkekse", "cracker", "knäckebrot"}, category: food, subcategory: "snacks", unit: "pct", shelfDays: 180},

		// ── Mercearia e temperos ─────────────────────────────────────────────
		{pt: "Açúcar", de: "Zucker", aliasesPt: []string{"acucar", "acucar refinado", "acucar cristal", "acucar mascavo", "acucar demerara"}, aliasesDe: []string{"zucker", "feiner zucker", "rohrzucker", "brauner zucker", "puderzucker"}, category: food, subcategory: "spices", unit: "kg", shelfDays: 720},
		{pt: "Sal", de: "Salz", aliasesPt: []string{"sal refinado", "sal grosso"}, aliasesDe: []string{"salz", "speisesalz", "meersalz", "jodsalz"}, category: food, subcategory: "spices", unit: "kg", shelfDays: 3650},
		{pt: "Óleo", de: "Öl", aliasesPt: []string{"oleo", "oleo de soja", "oleo de girassol", "oleo de milho"}, aliasesDe: []string{"öl", "oel", "sonnenblumenöl", "rapsöl", "speiseöl"}, category: food, subcategory: "spices", unit: "l", shelfDays: 365},
		{pt: "Azeite", de: "Olivenöl", aliasesPt: []string{"azeite de oliva", "azeite extra virgem"}, aliasesDe: []string{"olivenöl", "olivenoel", "natives olivenöl"}, category: food, subcategory: "spices", unit: "ml", shelfDays: 365},
		{pt: "Vinagre", de: "Essig", aliasesPt: []string{"vinagre de alcool", "vinagre de maca", "vinagre balsamico"}, aliasesDe: []string{"essig", "balsamico", "apfelessig", "weinessig"}, category: food, subcategory: "spices", unit: "ml", shelfDays: 720},
		{pt: "Molho de tomate", de: "Tomatensauce", aliasesPt: []string{"molho de tomate", "extrato de tomate", "polpa de tomate"}, aliasesDe: []string{"tomatensauce", "passierte tomaten", "tomatenmark", "gehackte tomaten", "pizzasauce"}, category: food, subcategory: "canned", unit: "un", shelfDays: 365},
		{pt: "Milho em lata", de: "Mais Dose", aliasesPt: []string{"milho verde lata", "milho conserva"}, aliasesDe: []string{"mais dose", "gemüsemais", "zuckermais dose"}, category: food, subcategory: "canned", unit: "un", shelfDays: 720},
		{pt: "Ervilha em lata", de: "Erbsen Dose", aliasesPt: []string{"ervilha", "ervilha conserva"}, aliasesDe: []string{"erbsen", "erbsen dose", "erbsen und möhren"}, category: food, subcategory: "canned", unit: "un", shelfDays: 720},
		{pt: "Azeitona", de: "Oliven", aliasesPt: []string{"azeitona verde", "azeitona preta"}, aliasesDe: []string{"oliven", "grüne oliven", "schwarze oliven"}, category: food, subcategory: "canned", unit: "un", shelfDays: 365},
		{pt: "Maionese", de: "Mayonnaise", aliasesDe: []string{"mayonnaise", "mayo", "salatmayonnaise"}, category: food, subcategory: "spices", unit: "un", shelfDays: 90},
		{pt: "Ketchup", de: "Ketchup", aliasesDe: []string{"ketchup", "tomatenketchup"}, category: food, subcategory: "spices", unit: "un", shelfDays: 180},
		{pt: "Mostarda", de: "Senf", aliasesDe: []string{"senf", "mittelscharfer senf"}, category: food, subcategory: "spices", unit: "un", shelfDays: 180},
		{pt: "Molho de soja", de: "Sojasauce", aliasesPt: []string{"shoyu", "molho shoyu"}, aliasesDe: []string{"sojasauce", "sojasoße"}, category: food, subcategory: "spices", unit: "ml", shelfDays: 365},
		{pt: "Pimenta-do-reino", de: "Pfeffer", aliasesPt: []string{"pimenta do reino", "pimenta preta"}, aliasesDe: []string{"pfeffer", "schwarzer pfeffer"}, category: food, subcategory: "spices", unit: "g", shelfDays: 720},
		{pt: "Orégano", de: "Oregano", aliasesPt: []string{"oregano"}, aliasesDe: []string{"oregano"}, category: food, subcategory: "spices", unit: "g", shelfDays: 720},
		{pt: "Canela", de: "Zimt", aliasesPt: []string{"canela em po", "canela em pau"}, aliasesDe: []string{"zimt"}, category: food, subcategory: "spices", unit: "g", shelfDays: 720},
		{pt: "Caldo de galinha", de: "Hühnerbrühe", aliasesPt: []string{"caldo knorr", "caldo de carne", "tempero pronto"}, aliasesDe: []string{"hühnerbrühe", "gemüsebrühe", "brühwürfel", "brühe"}, category: food, subcategory: "spices", unit: "un", shelfDays: 365},
		{pt: "Fermento", de: "Backpulver", aliasesPt: []string{"fermento em po", "fermento biologico", "fermento quimico"}, aliasesDe: []string{"backpulver", "hefe", "trockenhefe"}, category: food, subcategory: "spices", unit: "un", shelfDays: 365},
		{pt: "Mel", de: "Honig", aliasesDe: []string{"honig", "blütenhonig"}, category: food, subcategory: "spices", unit: "un", shelfDays: 720},
		{pt: "Geleia", de: "Marmelade", aliasesPt: []string{"geleia de morango", "geleia de uva"}, aliasesDe: []string{"marmelade", "konfitüre", "fruchtaufstrich"}, category: food, subcategory: "spices", unit: "un", shelfDays: 365},
		{pt: "Pasta de amendoim", de: "Erdnussbutter", aliasesDe: []string{"erdnussbutter", "erdnussmus"}, category: food, subcategory: "spices", unit: "un", shelfDays: 180},
		{pt: "Chocolate", de: "Schokolade", aliasesPt: []string{"chocolate ao leite", "barra de chocolate", "chocolate meio amargo"}, aliasesDe: []string{"schokolade", "vollmilch schokolade", "zartbitter", "tafel schokolade"}, category: food, subcategory: "snacks", unit: "un", shelfDays: 365},
		{pt: "Achocolatado", de: "Kakaopulver", aliasesPt: []string{"nescau", "toddy", "achocolatado em po"}, aliasesDe: []string{"kakao", "kakaopulver", "trinkschokolade"}, category: food, subcategory: "beverages", unit: "un", shelfDays: 365},
		{pt: "Salgadinho", de: "Chips", aliasesPt: []string{"batata chips", "salgadinho doritos", "amendoim"}, aliasesDe: []string{"chips", "kartoffelchips", "erdnüsse", "salzstangen"}, category: food, subcategory: "snacks", unit: "pct", shelfDays: 180},
		{pt: "Castanhas", de: "Nüsse", aliasesPt: []string{"castanha de caju", "castanha do para", "nozes", "amendoas", "mix de castanhas"}, aliasesDe: []string{"nüsse", "nuesse", "mandeln", "walnüsse", "cashewkerne", "studentenfutter"}, category: food, subcategory: "snacks", unit: "g", shelfDays: 180},

		// ── Congelados ───────────────────────────────────────────────────────
		{pt: "Pão de queijo congelado", de: "Käsebrötchen TK", aliasesPt: []string{"pao de queijo"}, aliasesDe: []string{"käsebrötchen"}, category: food, subcategory: "frozen", unit: "pct", shelfDays: 180},
		{pt: "Batata frita congelada", de: "Pommes TK", aliasesPt: []string{"batata palito", "batata pre frita"}, aliasesDe: []string{"pommes", "pommes frites", "tk pommes"}, category: food, subcategory: "frozen", unit: "pct", shelfDays: 365},
		{pt: "Pizza congelada", de: "Pizza TK", aliasesPt: []string{"pizza"}, aliasesDe: []string{"pizza", "tk pizza", "steinofenpizza"}, category: food, subcategory: "frozen", unit: "un", shelfDays: 365},
		{pt: "Legumes congelados", de: "TK Gemüse", aliasesPt: []string{"mix de legumes", "brocolis congelado", "ervilha congelada"}, aliasesDe: []string{"tk gemüse", "erbsen tk", "spinat tk", "gemüsemix"}, category: food, subcategory: "frozen", unit: "pct", shelfDays: 365},
		{pt: "Sorvete", de: "Eis", aliasesDe: []string{"eis", "eiscreme", "speiseeis"}, category: food, subcategory: "frozen", unit: "un", shelfDays: 180},

		// ── Bebidas ──────────────────────────────────────────────────────────
		{pt: "Café", de: "Kaffee", aliasesPt: []string{"cafe", "cafe em po", "cafe torrado e moido", "capsula de cafe"}, aliasesDe: []string{"kaffee", "kaffee gemahlen", "kaffeebohnen", "kaffeekapseln", "filterkaffee"}, category: food, subcategory: "beverages", unit: "g", shelfDays: 180},
		{pt: "Chá", de: "Tee", aliasesPt: []string{"cha", "cha mate", "cha de camomila"}, aliasesDe: []string{"tee", "schwarztee", "kräutertee", "grüner tee", "kamillentee"}, category: food, subcategory: "beverages", unit: "un", shelfDays: 365},
		{pt: "Água", de: "Wasser", aliasesPt: []string{"agua", "agua mineral", "agua com gas", "agua sem gas", "galao de agua"}, aliasesDe: []string{"wasser", "mineralwasser", "sprudel", "stilles wasser", "medium wasser"}, category: food, subcategory: "beverages", unit: "l", shelfDays: 365},
		{pt: "Suco", de: "Saft", aliasesPt: []string{"suco de laranja", "suco de uva", "suco integral", "nectar"}, aliasesDe: []string{"saft", "orangensaft", "apfelsaft", "o-saft", "apfelschorle", "multivitaminsaft"}, category: food, subcategory: "beverages", unit: "l", shelfDays: 30},
		{pt: "Refrigerante", de: "Limonade", aliasesPt: []string{"coca cola", "guarana", "fanta", "sprite", "pepsi"}, aliasesDe: []string{"cola", "coca cola", "fanta", "sprite", "limonade", "eistee"}, category: food, subcategory: "beverages", unit: "l", shelfDays: 180},
		{pt: "Cerveja", de: "Bier", aliasesPt: []string{"cerveja lata", "cerveja long neck", "heineken", "brahma", "skol"}, aliasesDe: []string{"bier", "pils", "weizen", "helles", "radler"}, category: food, subcategory: "alcohol", unit: "un", shelfDays: 180},
		{pt: "Vinho", de: "Wein", aliasesPt: []string{"vinho tinto", "vinho branco"}, aliasesDe: []string{"wein", "rotwein", "weißwein", "weisswein", "sekt"}, category: food, subcategory: "alcohol", unit: "un", shelfDays: 720},

		// ── Limpeza ──────────────────────────────────────────────────────────
		{pt: "Detergente", de: "Spülmittel", aliasesPt: []string{"detergente liquido", "detergente ype", "detergente neutro"}, aliasesDe: []string{"spülmittel", "spuelmittel", "spülm.", "spülm. lemon", "geschirrspülmittel"}, category: cleaning, subcategory: "kitchen", unit: "un"},
		{pt: "Sabão em pó", de: "Waschpulver", aliasesPt: []string{"sabao em po", "sabao omo", "lava roupas em po"}, aliasesDe: []string{"waschpulver", "waschmittel", "vollwaschmittel", "colorwaschmittel", "flüssigwaschmittel"}, category: cleaning, subcategory: "laundry", unit: "kg"},
		{pt: "Amaciante", de: "Weichspüler", aliasesDe: []string{"weichspüler", "weichspueler"}, category: cleaning, subcategory: "laundry", unit: "l"},
		{pt: "Água sanitária", de: "Bleichmittel", aliasesPt: []string{"agua sanitaria", "cloro", "qboa", "candida"}, aliasesDe: []string{"bleichmittel", "chlorreiniger", "dan klorix"}, category: cleaning, subcategory: "bathroom", unit: "l"},
		{pt: "Desinfetante", de: "Desinfektionsreiniger", aliasesPt: []string{"desinfetante pinho", "veja", "multiuso"}, aliasesDe: []string{"desinfektionsreiniger", "hygienereiniger", "sagrotan"}, category: cleaning, subcategory: "bathroom", unit: "l"},
		{pt: "Limpador multiuso", de: "Allzweckreiniger", aliasesPt: []string{"multiuso", "veja multiuso", "limpa vidros"}, aliasesDe: []string{"allzweckreiniger", "glasreiniger", "küchenreiniger", "badreiniger", "der general"}, category: cleaning, subcategory: "kitchen", unit: "un"},
		{pt: "Esponja de cozinha", de: "Schwamm", aliasesPt: []string{"esponja", "esponja scotch brite", "bucha"}, aliasesDe: []string{"schwamm", "schwämme", "topfreiniger", "spülschwamm"}, category: cleaning, subcategory: "kitchen", unit: "un"},
		{pt: "Saco de lixo", de: "Müllbeutel", aliasesPt: []string{"saco de lixo 50l", "saco de lixo 100l", "saco para lixo"}, aliasesDe: []string{"müllbeutel", "muellbeutel", "müllsäcke", "abfallbeutel"}, category: cleaning, subcategory: "kitchen", unit: "pct"},
		{pt: "Pano de limpeza", de: "Putztuch", aliasesPt: []string{"pano de chao", "pano multiuso", "perfex"}, aliasesDe: []string{"putztuch", "putztücher", "mikrofasertuch", "wischtuch"}, category: cleaning, subcategory: "kitchen", unit: "un"},
		{pt: "Lustra-móveis", de: "Möbelpolitur", aliasesPt: []string{"lustra moveis"}, aliasesDe: []string{"möbelpolitur"}, category: cleaning, subcategory: "kitchen", unit: "un"},
		{pt: "Sabão em barra", de: "Kernseife", aliasesPt: []string{"sabao em barra", "sabao de coco"}, aliasesDe: []string{"kernseife", "gallseife"}, category: cleaning, subcategory: "laundry", unit: "un"},
		{pt: "Tablete para lava-louças", de: "Spülmaschinentabs", aliasesPt: []string{"detergente lava loucas", "tablete lava loucas", "finish"}, aliasesDe: []string{"spülmaschinentabs", "geschirrspültabs", "tabs", "finish tabs"}, category: cleaning, subcategory: "kitchen", unit: "pct"},
		{pt: "Álcool", de: "Alkohol", aliasesPt: []string{"alcool", "alcool 70", "alcool gel"}, aliasesDe: []string{"alkohol", "desinfektionsmittel", "isopropanol"}, category: cleaning, subcategory: "bathroom", unit: "l"},

		// ── Higiene ──────────────────────────────────────────────────────────
		{pt: "Papel higiênico", de: "Toilettenpapier", aliasesPt: []string{"papel higienico", "papel higienico folha dupla"}, aliasesDe: []string{"toilettenpapier", "klopapier", "wc papier"}, category: hygiene, subcategory: "paper", unit: "un"},
		{pt: "Papel toalha", de: "Küchenrolle", aliasesPt: []string{"papel toalha", "toalha de papel"}, aliasesDe: []string{"küchenrolle", "kuechenrolle", "küchentücher", "haushaltstücher"}, category: hygiene, subcategory: "paper", unit: "un"},
		{pt: "Lenço de papel", de: "Taschentücher", aliasesPt: []string{"lenco de papel", "lencinho"}, aliasesDe: []string{"taschentücher", "taschentuecher", "tempo"}, category: hygiene, subcategory: "paper", unit: "pct"},
		{pt: "Sabonete", de: "Seife", aliasesPt: []string{"sabonete em barra", "sabonete liquido", "dove"}, aliasesDe: []string{"seife", "duschgel", "flüssigseife", "handseife", "duschbad"}, category: hygiene, subcategory: "body", unit: "un"},
		{pt: "Shampoo", de: "Shampoo", aliasesPt: []string{"xampu"}, aliasesDe: []string{"shampoo", "haarshampoo"}, category: hygiene, subcategory: "hair", unit: "un"},
		{pt: "Condicionador", de: "Spülung", aliasesDe: []string{"spülung", "spuelung", "conditioner", "haarspülung"}, category: hygiene, subcategory: "hair", unit: "un"},
		{pt: "Pasta de dente", de: "Zahnpasta", aliasesPt: []string{"creme dental", "colgate", "pasta dental"}, aliasesDe: []string{"zahnpasta", "zahncreme"}, category: hygiene, subcategory: "oral", unit: "un"},
		{pt: "Escova de dente", de: "Zahnbürste", aliasesDe: []string{"zahnbürste", "zahnbuerste"}, category: hygiene, subcategory: "oral", unit: "un"},
		{pt: "Fio dental", de: "Zahnseide", aliasesDe: []string{"zahnseide"}, category: hygiene, subcategory: "oral", unit: "un"},
		{pt: "Enxaguante bucal", de: "Mundspülung", aliasesPt: []string{"listerine", "enxaguatorio bucal"}, aliasesDe: []string{"mundspülung", "mundwasser"}, category: hygiene, subcategory: "oral", unit: "un"},
		{pt: "Desodorante", de: "Deo", aliasesPt: []string{"desodorante aerosol", "desodorante roll on", "rexona"}, aliasesDe: []string{"deo", "deodorant", "deo spray", "deo roll-on"}, category: hygiene, subcategory: "body", unit: "un"},
		{pt: "Absorvente", de: "Binden", aliasesPt: []string{"absorvente intimo", "absorvente always"}, aliasesDe: []string{"binden", "tampons", "slipeinlagen"}, category: hygiene, subcategory: "body", unit: "pct"},
		{pt: "Hidratante", de: "Bodylotion", aliasesPt: []string{"creme hidratante", "locao hidratante"}, aliasesDe: []string{"bodylotion", "körperlotion", "handcreme", "creme"}, category: hygiene, subcategory: "body", unit: "un"},
		{pt: "Protetor solar", de: "Sonnencreme", aliasesDe: []string{"sonnencreme", "sonnenmilch", "sonnenschutz"}, category: hygiene, subcategory: "body", unit: "un"},
		{pt: "Cotonete", de: "Wattestäbchen", aliasesPt: []string{"haste flexivel"}, aliasesDe: []string{"wattestäbchen", "wattestaebchen"}, category: hygiene, subcategory: "body", unit: "pct"},
		{pt: "Lâmina de barbear", de: "Rasierklingen", aliasesPt: []string{"lamina de barbear", "gillette", "aparelho de barbear"}, aliasesDe: []string{"rasierklingen", "rasierer", "einwegrasierer"}, category: hygiene, subcategory: "body", unit: "pct"},

		// ── Geral / casa ─────────────────────────────────────────────────────
		{pt: "Papel alumínio", de: "Alufolie", aliasesPt: []string{"papel aluminio"}, aliasesDe: []string{"alufolie", "aluminiumfolie"}, category: general, subcategory: "kitchen", unit: "un"},
		{pt: "Filme plástico", de: "Frischhaltefolie", aliasesPt: []string{"filme plastico", "plastico filme", "papel filme"}, aliasesDe: []string{"frischhaltefolie"}, category: general, subcategory: "kitchen", unit: "un"},
		{pt: "Guardanapo", de: "Servietten", aliasesPt: []string{"guardanapo de papel"}, aliasesDe: []string{"servietten"}, category: general, subcategory: "paper", unit: "pct"},
		{pt: "Filtro de café", de: "Kaffeefilter", aliasesPt: []string{"filtro de papel", "coador de papel"}, aliasesDe: []string{"kaffeefilter", "filtertüten"}, category: general, subcategory: "kitchen", unit: "pct"},
		{pt: "Pilha", de: "Batterien", aliasesPt: []string{"pilha aa", "pilha aaa", "pilhas"}, aliasesDe: []string{"batterien", "batterie aa", "batterie aaa"}, category: general, subcategory: "other", unit: "pct"},
		{pt: "Lâmpada", de: "Glühbirne", aliasesPt: []string{"lampada", "lampada led"}, aliasesDe: []string{"glühbirne", "led lampe", "leuchtmittel"}, category: general, subcategory: "other", unit: "un"},
		{pt: "Fósforo", de: "Streichhölzer", aliasesPt: []string{"fosforo", "isqueiro"}, aliasesDe: []string{"streichhölzer", "feuerzeug"}, category: general, subcategory: "other", unit: "un"},
		{pt: "Ração para pet", de: "Tierfutter", aliasesPt: []string{"racao", "racao cachorro", "racao gato", "sache"}, aliasesDe: []string{"tierfutter", "hundefutter", "katzenfutter", "trockenfutter"}, category: general, subcategory: "pet", unit: "kg"},
		{pt: "Areia para gato", de: "Katzenstreu", aliasesPt: []string{"areia sanitaria", "areia de gato"}, aliasesDe: []string{"katzenstreu"}, category: general, subcategory: "pet", unit: "kg"},
		{pt: "Fralda", de: "Windeln", aliasesPt: []string{"fralda descartavel", "pampers"}, aliasesDe: []string{"windeln", "pampers"}, category: general, subcategory: "baby", unit: "pct"},
		{pt: "Lenço umedecido", de: "Feuchttücher", aliasesPt: []string{"lenco umedecido", "lencos umedecidos"}, aliasesDe: []string{"feuchttücher", "feuchttuecher"}, category: general, subcategory: "baby", unit: "pct"},
	}
}
