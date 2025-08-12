package game

// UnitSnapshot captures the minimal stats needed for resolution
type UnitSnapshot struct {
    ID    string
    Name  string
    T     int // toughness
    W     int // total wounds
    Sv    int // armor save (2-6; 7 means none)
    InvSv int // invulnerable save (2-6; 0 if none)
    Keywords []string // unit keywords (e.g., Infantry, Vehicle)
    Abilities []string // unit abilities (e.g., Feel No Pain 5+)
}

// WeaponSnapshot for a single weapon profile
type WeaponSnapshot struct {
    Name       string
    Type       string // "melee" or "ranged"
    Attacks    string // dice expr or int
    Skill      int    // hit threshold (2-6)
    Strength   int
    AP         int // e.g., -1 means worsen save by 1
    Damage     string // dice expr or int
    Abilities  []string // normalized ability tokens from weapon profile
}

// ShootingResult captures outcome and logs
type ShootingResult struct {
    Logs           []string `json:"logs"`
    Attacks        int      `json:"attacks"`
    Hits           int      `json:"hits"`
    Wounds         int      `json:"wounds"`
    Saved          int      `json:"saved"`
    Unsaved        int      `json:"unsaved"`
    DamageTotal    int      `json:"damage_total"`
    DefenderWounds int      `json:"defender_wounds"`
    // Optional structured breakdown into sub-phases for UI/analysis
    Subphases      *ShootingSubphases `json:"subphases,omitempty"`
}

// ShootingSubphases describes phase-by-phase rolls & targets
type ShootingSubphases struct {
    Attacks struct {
        Count int `json:"count"`
    } `json:"attacks"`
    Hits struct {
        Target  int   `json:"target"`
        Rolls   []int `json:"rolls"`
        Success int   `json:"success"`
    } `json:"hits"`
    Wounds struct {
        Target  int   `json:"target"`
        Rolls   []int `json:"rolls"`
        Success int   `json:"success"`
    } `json:"wounds"`
    Saves struct {
        Target  int   `json:"target"`
        Rolls   []int `json:"rolls"`
        Success int   `json:"success"`
        Failed  int   `json:"failed"`
    } `json:"saves"`
    Damage struct {
        Rolls []int `json:"rolls"`
        Total int   `json:"total"`
    } `json:"damage"`
}
