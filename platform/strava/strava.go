package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

const tokenFile = "strava_token.json"

// TokenManager holds the configuration and token for interacting with the Strava API.
type TokenManager struct {
	Config *oauth2.Config
	Token  *oauth2.Token
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() *Client {
	stravaManager, err := NewTokenManager()
	if err != nil {
		log.Fatalf("Failed to create token manager: %v", err)
	}

	if stravaManager.Token == nil || stravaManager.Token.AccessToken == "" {
		stravaManager.StartLoginWebServer()
	}

	client, err := stravaManager.GetClient()
	if err != nil {
		log.Fatalf("Failed to get authenticated client: %v", err)
	}

	return &Client{
		client,
		"https://www.strava.com/api/v3",
	}

}

func (c Client) GetAthlete() Athlete {
	resp, err := c.httpClient.Get(c.baseURL + "/athlete")

	if err != nil {
		log.Fatalf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	fmt.Printf("\nAPI Response:\n%s\n", string(body))

	var athlete Athlete
	err = json.Unmarshal(body, &athlete)
	if err != nil {
		log.Fatalf("Failed to unmarshal response body: %v", err)
	}
	return athlete
}

func (c Client) GetActivity(activityId int64) Activity {
	resp, err := c.httpClient.Get(c.baseURL + "/activities/" + strconv.FormatInt(activityId, 10))
	if err != nil {
		log.Fatalf("API call failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}
	var activity Activity
	err = json.Unmarshal(body, &activity)
	if err != nil {
		log.Fatalf("Failed to unmarshal response body: %v", err)
	}
	return activity
}

// NewTokenManager creates a manager, gets credentials from env vars, and loads a token if it exists.
func NewTokenManager() (*TokenManager, error) {
	clientID := os.Getenv("STRAVA_CLIENT_ID")
	clientSecret := os.Getenv("STRAVA_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET must be set")
	}

	tm := &TokenManager{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  "http://localhost:8080/callback",
			Scopes:       []string{"read_all,activity:read_all,profile:read_all"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.strava.com/oauth/authorize",
				TokenURL: "https://www.strava.com/oauth/token",
			},
		},
	}

	// Try to load a token from the file.
	if err := tm.loadToken(); err != nil {
		log.Printf("Token file not found or invalid: %v. Ready for new login.", err)
	}
	return tm, nil
}

// saveToken persists the token to the local file.
func (tm *TokenManager) saveToken() error {
	file, err := json.MarshalIndent(tm.Token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenFile, file, 0600)
}

// loadToken retrieves the token from the local file.
func (tm *TokenManager) loadToken() error {
	file, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, &tm.Token)
}

// GetClient ensures the token is valid (refreshing if needed) and returns an http.Client.
func (tm *TokenManager) GetClient() (*http.Client, error) {
	if tm.Token == nil || tm.Token.AccessToken == "" {
		return nil, fmt.Errorf("no token available, please login first")
	}

	tokenSource := tm.Config.TokenSource(context.Background(), tm.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get or refresh token: %w", err)
	}

	// If the token was refreshed, save the new one.
	if newToken.AccessToken != tm.Token.AccessToken {
		log.Println("Token was refreshed.")
		tm.Token = newToken
		if err := tm.saveToken(); err != nil {
			log.Printf("Warning: failed to save refreshed token: %v", err)
		}
	}

	return tm.Config.Client(context.Background(), tm.Token), nil
}

// StartLoginWebServer handles the initial OAuth flow if no token exists.
func (tm *TokenManager) StartLoginWebServer() {
	// A simple state string for CSRF protection.
	oauthStateString := "pseudo-random"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := tm.Config.AuthCodeURL(oauthStateString)
		html := fmt.Sprintf(`<html><body><a href="%s">Login with Strava</a></body></html>`, url)
		fmt.Fprint(w, html)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("state") != oauthStateString {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		token, err := tm.Config.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Code exchange failed: %s", err), http.StatusInternalServerError)
			return
		}

		tm.Token = token
		if err := tm.saveToken(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save token: %s", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, "<h1>Login Successful!</h1><p>Token saved. You can now close this window and restart the application.</p>")
	})

	fmt.Println("No token found. Please open http://localhost:8080 to authorize.")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Athlete struct {
	Id                    int         `json:"id"`
	Username              interface{} `json:"username"`
	ResourceState         int         `json:"resource_state"`
	Firstname             string      `json:"firstname"`
	Lastname              string      `json:"lastname"`
	Bio                   string      `json:"bio"`
	City                  string      `json:"city"`
	State                 string      `json:"state"`
	Country               string      `json:"country"`
	Sex                   string      `json:"sex"`
	Premium               bool        `json:"premium"`
	Summit                bool        `json:"summit"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
	BadgeTypeId           int         `json:"badge_type_id"`
	Weight                float64     `json:"weight"`
	ProfileMedium         string      `json:"profile_medium"`
	Profile               string      `json:"profile"`
	Friend                interface{} `json:"friend"`
	Follower              interface{} `json:"follower"`
	Blocked               bool        `json:"blocked"`
	CanFollow             bool        `json:"can_follow"`
	FollowerCount         int         `json:"follower_count"`
	FriendCount           int         `json:"friend_count"`
	MutualFriendCount     int         `json:"mutual_friend_count"`
	AthleteType           int         `json:"athlete_type"`
	DatePreference        string      `json:"date_preference"`
	MeasurementPreference string      `json:"measurement_preference"`
	Clubs                 []struct {
		Id                 int      `json:"id"`
		ResourceState      int      `json:"resource_state"`
		Name               string   `json:"name"`
		ProfileMedium      string   `json:"profile_medium"`
		Profile            string   `json:"profile"`
		CoverPhoto         string   `json:"cover_photo"`
		CoverPhotoSmall    string   `json:"cover_photo_small"`
		ActivityTypes      []string `json:"activity_types"`
		ActivityTypesIcon  string   `json:"activity_types_icon"`
		Dimensions         []string `json:"dimensions"`
		SportType          string   `json:"sport_type"`
		LocalizedSportType string   `json:"localized_sport_type"`
		City               string   `json:"city"`
		State              string   `json:"state"`
		Country            string   `json:"country"`
		Private            bool     `json:"private"`
		MemberCount        int      `json:"member_count"`
		Featured           bool     `json:"featured"`
		Verified           bool     `json:"verified"`
		Url                string   `json:"url"`
		Membership         string   `json:"membership"`
		Admin              bool     `json:"admin"`
		Owner              bool     `json:"owner"`
	} `json:"clubs"`
	PostableClubsCount int         `json:"postable_clubs_count"`
	Ftp                interface{} `json:"ftp"`
	Bikes              []struct {
		Id                string  `json:"id"`
		Primary           bool    `json:"primary"`
		Name              string  `json:"name"`
		Nickname          string  `json:"nickname"`
		ResourceState     int     `json:"resource_state"`
		Retired           bool    `json:"retired"`
		Distance          int     `json:"distance"`
		ConvertedDistance float64 `json:"converted_distance"`
	} `json:"bikes"`
	Shoes []struct {
		Id                string      `json:"id"`
		Primary           bool        `json:"primary"`
		Name              string      `json:"name"`
		Nickname          interface{} `json:"nickname"`
		ResourceState     int         `json:"resource_state"`
		Retired           bool        `json:"retired"`
		Distance          int         `json:"distance"`
		ConvertedDistance float64     `json:"converted_distance"`
	} `json:"shoes"`
	IsWinbackViaUpload bool `json:"is_winback_via_upload"`
	IsWinbackViaView   bool `json:"is_winback_via_view"`
}
type Activity struct {
	Id            int64  `json:"id"`
	ResourceState int    `json:"resource_state"`
	ExternalId    string `json:"external_id"`
	UploadId      int64  `json:"upload_id"`
	Athlete       struct {
		Id            int `json:"id"`
		ResourceState int `json:"resource_state"`
	} `json:"athlete"`
	Name               string    `json:"name"`
	Distance           float64   `json:"distance"`
	MovingTime         int       `json:"moving_time"`
	ElapsedTime        int       `json:"elapsed_time"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
	Type               string    `json:"type"`
	SportType          string    `json:"sport_type"`
	StartDate          time.Time `json:"start_date"`
	StartDateLocal     time.Time `json:"start_date_local"`
	Timezone           string    `json:"timezone"`
	UtcOffset          float64   `json:"utc_offset"`
	StartLatlng        []float64 `json:"start_latlng"`
	EndLatlng          []float64 `json:"end_latlng"`
	AchievementCount   int       `json:"achievement_count"`
	KudosCount         int       `json:"kudos_count"`
	CommentCount       int       `json:"comment_count"`
	AthleteCount       int       `json:"athlete_count"`
	PhotoCount         int       `json:"photo_count"`
	Map                struct {
		Id              string `json:"id"`
		Polyline        string `json:"polyline"`
		ResourceState   int    `json:"resource_state"`
		SummaryPolyline string `json:"summary_polyline"`
	} `json:"map"`
	Trainer              bool        `json:"trainer"`
	Commute              bool        `json:"commute"`
	Manual               bool        `json:"manual"`
	Private              bool        `json:"private"`
	Flagged              bool        `json:"flagged"`
	GearId               string      `json:"gear_id"`
	FromAcceptedTag      bool        `json:"from_accepted_tag"`
	AverageSpeed         float64     `json:"average_speed"`
	MaxSpeed             float64     `json:"max_speed"`
	AverageCadence       float64     `json:"average_cadence"`
	AverageTemp          int         `json:"average_temp"`
	AverageWatts         float64     `json:"average_watts"`
	WeightedAverageWatts int         `json:"weighted_average_watts"`
	Kilojoules           float64     `json:"kilojoules"`
	DeviceWatts          bool        `json:"device_watts"`
	HasHeartrate         bool        `json:"has_heartrate"`
	MaxWatts             int         `json:"max_watts"`
	ElevHigh             float64     `json:"elev_high"`
	ElevLow              float64     `json:"elev_low"`
	PrCount              int         `json:"pr_count"`
	TotalPhotoCount      int         `json:"total_photo_count"`
	HasKudoed            bool        `json:"has_kudoed"`
	WorkoutType          int         `json:"workout_type"`
	SufferScore          interface{} `json:"suffer_score"`
	Description          string      `json:"description"`
	Calories             float64     `json:"calories"`
	SegmentEfforts       []struct {
		Id            int64  `json:"id"`
		ResourceState int    `json:"resource_state"`
		Name          string `json:"name"`
		Activity      struct {
			Id            int64 `json:"id"`
			ResourceState int   `json:"resource_state"`
		} `json:"activity"`
		Athlete struct {
			Id            int `json:"id"`
			ResourceState int `json:"resource_state"`
		} `json:"athlete"`
		ElapsedTime    int       `json:"elapsed_time"`
		MovingTime     int       `json:"moving_time"`
		StartDate      time.Time `json:"start_date"`
		StartDateLocal time.Time `json:"start_date_local"`
		Distance       float64   `json:"distance"`
		StartIndex     int       `json:"start_index"`
		EndIndex       int       `json:"end_index"`
		AverageCadence float64   `json:"average_cadence"`
		DeviceWatts    bool      `json:"device_watts"`
		AverageWatts   float64   `json:"average_watts"`
		Segment        struct {
			Id            int       `json:"id"`
			ResourceState int       `json:"resource_state"`
			Name          string    `json:"name"`
			ActivityType  string    `json:"activity_type"`
			Distance      float64   `json:"distance"`
			AverageGrade  float64   `json:"average_grade"`
			MaximumGrade  float64   `json:"maximum_grade"`
			ElevationHigh float64   `json:"elevation_high"`
			ElevationLow  float64   `json:"elevation_low"`
			StartLatlng   []float64 `json:"start_latlng"`
			EndLatlng     []float64 `json:"end_latlng"`
			ClimbCategory int       `json:"climb_category"`
			City          string    `json:"city"`
			State         string    `json:"state"`
			Country       string    `json:"country"`
			Private       bool      `json:"private"`
			Hazardous     bool      `json:"hazardous"`
			Starred       bool      `json:"starred"`
		} `json:"segment"`
		KomRank      interface{}   `json:"kom_rank"`
		PrRank       interface{}   `json:"pr_rank"`
		Achievements []interface{} `json:"achievements"`
		Hidden       bool          `json:"hidden"`
	} `json:"segment_efforts"`
	SplitsMetric []struct {
		Distance            float64 `json:"distance"`
		ElapsedTime         int     `json:"elapsed_time"`
		ElevationDifference float64 `json:"elevation_difference"`
		MovingTime          int     `json:"moving_time"`
		Split               int     `json:"split"`
		AverageSpeed        float64 `json:"average_speed"`
		PaceZone            int     `json:"pace_zone"`
	} `json:"splits_metric"`
	Laps []struct {
		Id            int64  `json:"id"`
		ResourceState int    `json:"resource_state"`
		Name          string `json:"name"`
		Activity      struct {
			Id            int `json:"id"`
			ResourceState int `json:"resource_state"`
		} `json:"activity"`
		Athlete struct {
			Id            int `json:"id"`
			ResourceState int `json:"resource_state"`
		} `json:"athlete"`
		ElapsedTime        int       `json:"elapsed_time"`
		MovingTime         int       `json:"moving_time"`
		StartDate          time.Time `json:"start_date"`
		StartDateLocal     time.Time `json:"start_date_local"`
		Distance           float64   `json:"distance"`
		StartIndex         int       `json:"start_index"`
		EndIndex           int       `json:"end_index"`
		TotalElevationGain float64   `json:"total_elevation_gain"`
		AverageSpeed       float64   `json:"average_speed"`
		MaxSpeed           float64   `json:"max_speed"`
		AverageCadence     float64   `json:"average_cadence"`
		DeviceWatts        bool      `json:"device_watts"`
		AverageWatts       float64   `json:"average_watts"`
		LapIndex           int       `json:"lap_index"`
		Split              float64   `json:"split"`
	} `json:"laps"`
	Gear struct {
		Id            string `json:"id"`
		Primary       bool   `json:"primary"`
		Name          string `json:"name"`
		ResourceState int    `json:"resource_state"`
		Distance      int    `json:"distance"`
	} `json:"gear"`
	PartnerBrandTag interface{} `json:"partner_brand_tag"`
	Photos          struct {
		Primary struct {
			Id       interface{} `json:"id"`
			UniqueId string      `json:"unique_id"`
			Urls     struct {
				Field1 string `json:"100"`
				Field2 string `json:"600"`
			} `json:"urls"`
			Source int `json:"source"`
		} `json:"primary"`
		UsePrimaryPhoto bool `json:"use_primary_photo"`
		Count           int  `json:"count"`
	} `json:"photos"`
	HighlightedKudosers []struct {
		DestinationUrl string `json:"destination_url"`
		DisplayName    string `json:"display_name"`
		AvatarUrl      string `json:"avatar_url"`
		ShowName       bool   `json:"show_name"`
	} `json:"highlighted_kudosers"`
	HideFromHome             bool   `json:"hide_from_home"`
	DeviceName               string `json:"device_name"`
	EmbedToken               string `json:"embed_token"`
	SegmentLeaderboardOptOut bool   `json:"segment_leaderboard_opt_out"`
	LeaderboardOptOut        bool   `json:"leaderboard_opt_out"`
}
