package location

type City struct {
	Name string
	Lat  float64
	Lon  float64
}

func DefaultEUCities() []City {
	return []City{
		{Name: "Berlin", Lat: 52.52, Lon: 13.4050},
		{Name: "Paris", Lat: 48.8566, Lon: 2.3522},
		{Name: "Madrid", Lat: 40.4168, Lon: -3.7038},
		{Name: "Rome", Lat: 41.9028, Lon: 12.4964},
		{Name: "Vienna", Lat: 48.2082, Lon: 16.3738},
		{Name: "Prague", Lat: 50.0755, Lon: 14.4378},
		{Name: "Warsaw", Lat: 52.2297, Lon: 21.0122},
		{Name: "Budapest", Lat: 47.4979, Lon: 19.0402},
		{Name: "Amsterdam", Lat: 52.3676, Lon: 4.9041},
		{Name: "Brussels", Lat: 50.8503, Lon: 4.3517},
		{Name: "Copenhagen", Lat: 55.6761, Lon: 12.5683},
		{Name: "Stockholm", Lat: 59.3293, Lon: 18.0686},
		{Name: "Oslo", Lat: 59.9139, Lon: 10.7522},
		{Name: "Helsinki", Lat: 60.1699, Lon: 24.9384},
		{Name: "Dublin", Lat: 53.3498, Lon: -6.2603},
		{Name: "Lisbon", Lat: 38.7223, Lon: -9.1393},
		{Name: "Athens", Lat: 37.9838, Lon: 23.7275},
		{Name: "Zurich", Lat: 47.3769, Lon: 8.5417},
		{Name: "Munich", Lat: 48.1351, Lon: 11.5820},
		{Name: "Hamburg", Lat: 53.5511, Lon: 9.9937},
		{Name: "Kyiv", Lat: 50.4501, Lon: 30.5234},
		{Name: "Lviv", Lat: 49.8397, Lon: 24.0297},
		{Name: "London", Lat: 51.5074, Lon: -0.1278},
		{Name: "Milan", Lat: 45.4642, Lon: 9.1900},
		{Name: "Barcelona", Lat: 41.3851, Lon: 2.1734},
	}
}
