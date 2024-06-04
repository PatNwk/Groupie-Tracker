package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Artiste représente les informations sur un artiste.
type Artiste struct {
	ID           int      `json:"id"`
	Image        string   `json:"image"`
	Name         string   `json:"name"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
}

// Relation représente les informations sur les relations.
type Relation struct {
	Index []struct {
		Id             int                 `json:"id"`
		DatesLocations map[string][]string `json:"datesLocations"`
	} `json:"index"`
}

// ArtistsHTML est une structure pour passer des données au modèle
type ArtistsHTML struct {
	Artists        []Artiste
	Relations      Relation
	Pays           []string // Liste des pays
	ArtistNames    []string // Liste des noms d'artiste
	ArtistMembers  []string // Liste des membres d'artiste
	ArtistCreation []int    // Liste des dates de création d'artiste
	ArtistSearch   string   // Chaîne de recherche d'artiste
	MemberSearch   string   // Chaîne de recherche de membre
}

const apiURL = "https://groupietrackers.herokuapp.com/api/artists"
const apiURL2 = "https://groupietrackers.herokuapp.com/api/relation"

var tmpl = template.Must(template.ParseFiles("index.html"))

func fetchData(url string, target interface{}) error {
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Erreur lors de la requête HTTP : %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Réponse non OK de l'API : %s", response.Status)
	}

	err = json.NewDecoder(response.Body).Decode(target)
	if err != nil {
		return fmt.Errorf("Erreur lors du décodage JSON : %v", err)
	}

	return nil
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("searchQuery")

	// Récupérer la chaîne de recherche de membre depuis la requête
	memberQuery := r.URL.Query().Get("memberSearch")

	// Récupérer les paramètres de tri depuis la requête
	sortBy := r.URL.Query().Get("sortBy")
	orderBy := r.URL.Query().Get("orderBy")

	// Récupérer les dates minimale et maximale depuis la requête
	minYearStr := r.URL.Query().Get("minYear")
	maxYearStr := r.URL.Query().Get("maxYear")

	minYear, err := strconv.Atoi(minYearStr)
	if err != nil {
		minYear = 0 // Valeur par défaut si la conversion échoue
	}

	maxYear, err := strconv.Atoi(maxYearStr)
	if err != nil {
		maxYear = 9999 // Valeur par défaut si la conversion échoue
	}

	// Récupérer les valeurs des cases cochées pour le nombre de membres
	numMembersFilters := r.URL.Query()["numMembers"]

	// Récupérer le pays sélectionné depuis la requête
	selectedCountry := r.URL.Query().Get("country")

	// Récupérer les données depuis l'API des relations
	var relations Relation
	err = fetchData(apiURL2, &relations)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de la récupération des données depuis l'API des relations: %v", err), http.StatusInternalServerError)
		return
	}

	// Récupérer les données depuis l'API des artistes
	var artists []Artiste
	err = fetchData(apiURL, &artists)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de la récupération des données depuis l'API des artistes: %v", err), http.StatusInternalServerError)
		return
	}

	var filteredArtists []Artiste
	for _, artist := range artists {
		// Rechercher dans le nom de l'artiste
		if searchQuery != "" && strings.Contains(strings.ToLower(artist.Name), strings.ToLower(searchQuery)) {
			filteredArtists = append(filteredArtists, artist)
		} else {
			for _, member := range artist.Members {
				if strings.Contains(strings.ToLower(member), strings.ToLower(searchQuery)) {
					filteredArtists = append(filteredArtists, artist)
					break // Ajouter l'artiste une seule fois
				}
			}
		}
	}

	// Appliquer la recherche par membre
	if memberQuery != "" {
		var artistsWithMember []Artiste
		for _, artist := range filteredArtists {
			for _, member := range artist.Members {
				if strings.Contains(strings.ToLower(member), strings.ToLower(memberQuery)) {
					artistsWithMember = append(artistsWithMember, artist)
					break // Ajouter l'artiste une seule fois
				}
			}
		}
		filteredArtists = artistsWithMember
	}

	// Appliquer le filtrage par nombre de membres à la liste filtrée
	var filteredByNumMembers []Artiste
	if len(numMembersFilters) > 0 {
		for _, artist := range filteredArtists {
			for _, filter := range numMembersFilters {
				numMembers, err := strconv.Atoi(filter)
				if err != nil {
					continue
				}
				// Compter le nombre de membres de l'artiste
				if len(artist.Members) == numMembers {
					filteredByNumMembers = append(filteredByNumMembers, artist)
				}
			}
		}
	} else {
		filteredByNumMembers = filteredArtists
	}

	// Appliquer le filtrage par date minimale et maximale de création
	var filteredByDate []Artiste
	for _, artist := range filteredByNumMembers {
		if artist.CreationDate >= minYear && artist.CreationDate <= maxYear {
			filteredByDate = append(filteredByDate, artist)
		}
	}

	// Filtrer les artistes par pays si un pays est sélectionné
	var filteredByCountry []Artiste
	if selectedCountry != "" {
		for _, artist := range filteredByDate {
			// Vérifier si l'artiste est associé à la localisation sélectionnée
			found := false
			for _, relation := range relations.Index {
				locations, ok := relation.DatesLocations[artist.Name]
				if ok {
					for _, location := range locations {
						parts := strings.Split(location, ",")
						if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == selectedCountry {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			// Ajouter l'artiste à la liste filtrée s'il est associé à la localisation sélectionnée
			if found {
				filteredByCountry = append(filteredByCountry, artist)
			}
		}
	} else {
		filteredByCountry = filteredByDate
	}

	// Trier les artistes en fonction des critères de tri spécifiés
	sort.SliceStable(filteredByCountry, func(i, j int) bool {
		switch sortBy {
		case "creationDate":
			switch orderBy {
			case "asc":
				return filteredByCountry[i].CreationDate < filteredByCountry[j].CreationDate
			case "desc":
				return filteredByCountry[i].CreationDate > filteredByCountry[j].CreationDate
			}
		case "firstAlbum":
			switch orderBy {
			case "asc":
				return parseYear(filteredByCountry[i].FirstAlbum) < parseYear(filteredByCountry[j].FirstAlbum)
			case "desc":
				return parseYear(filteredByCountry[i].FirstAlbum) > parseYear(filteredByCountry[j].FirstAlbum)
			}
		case "country":
			// Trie par nom de pays
			country1 := getCountryForArtist(filteredByCountry[i].Name, relations)
			country2 := getCountryForArtist(filteredByCountry[j].Name, relations)
			switch orderBy {
			case "asc":
				return country1 < country2
			case "desc":
				return country1 > country2
			}
			// Par défaut, trier par nom si les critères de tri sont les mêmes
		}
		return filteredByCountry[i].Name < filteredByCountry[j].Name
	})

	// Extraire les pays de localisation uniques à partir des relations
	countries := extractUniqueCountriesFromRelations(relations)

	// Passer les données au modèle
	err = tmpl.Execute(w, ArtistsHTML{
		Artists:        filteredByCountry,
		Relations:      relations,
		Pays:           countries,
		ArtistNames:    getArtistNames(filteredByCountry),
		ArtistMembers:  getArtistMembers(filteredByCountry),
		ArtistCreation: getArtistCreationDates(filteredByCountry),
		MemberSearch:   memberQuery,
		ArtistSearch:   searchQuery,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de l'exécution du modèle: %v", err), http.StatusInternalServerError)
		return
	}
}

func parseYear(date string) int {
	if date == "" {
		return 0
	}
	t, err := time.Parse("02-01-2006", date)
	if err != nil {
		fmt.Printf("Erreur lors de l'analyse de la date : %v\n", err)
		return 0
	}
	return t.Year()
}

func extractUniqueCountriesFromRelations(relations Relation) []string {
	countryMap := make(map[string]bool)
	for _, relation := range relations.Index {
		for date := range relation.DatesLocations {
			parts := strings.Split(date, ",")
			if len(parts) > 0 {
				countryMap[strings.TrimSpace(parts[len(parts)-1])] = true
			}
		}
	}
	countries := make([]string, 0, len(countryMap))
	for country := range countryMap {
		countries = append(countries, country)
	}
	sort.Strings(countries)
	return countries
}

// Fonction pour obtenir le pays associé à un artiste à partir des relations
func getCountryForArtist(artistName string, relations Relation) string {
	for _, relation := range relations.Index {
		locations, ok := relation.DatesLocations[artistName]
		if ok && len(locations) > 0 {
			parts := strings.Split(locations[0], ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return "" // Retourne une chaîne vide si le pays n'est pas trouvé
}

// Fonction pour obtenir les noms d'artiste
func getArtistNames(artists []Artiste) []string {
	names := make([]string, len(artists))
	for i, artist := range artists {
		names[i] = artist.Name
	}
	return names
}

// Fonction pour obtenir les membres d'artiste
func getArtistMembers(artists []Artiste) []string {
	var members []string
	for _, artist := range artists {
		members = append(members, artist.Members...)
	}
	return members
}

// Fonction pour obtenir les dates de création d'artiste
func getArtistCreationDates(artists []Artiste) []int {
	dates := make([]int, len(artists))
	for i, artist := range artists {
		dates[i] = artist.CreationDate
	}
	return dates
}

func main() {
	http.HandleFunc("/", handleRequest)
	http.ListenAndServe(":8080", nil)
}
