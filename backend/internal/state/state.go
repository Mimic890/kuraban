package state

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

const CurrentSecurityVersion = 1

type PasskeyCredential struct {
	Credential webauthn.Credential `json:"credential"`
	Name       string              `json:"name"`
	CreatedAt  time.Time           `json:"created_at"`
	LastUsedAt time.Time           `json:"last_used_at,omitempty"`
}

type AdminAccount struct {
	Nickname             string              `json:"nickname"`
	Password             string              `json:"password"` // SHA-256 then bcrypt, empty if no password set yet
	Setup                bool                `json:"setup"`
	Email                string              `json:"email,omitempty"`
	SecurityVersion      int                 `json:"security_version,omitempty"`
	WebAuthnUserHandle   []byte              `json:"webauthn_user_handle,omitempty"`
	TOTPSecret           string              `json:"totp_secret,omitempty"`
	TOTPLastAcceptedStep int64               `json:"totp_last_accepted_timestep,omitempty"`
	RecoveryCodeHashes   []string            `json:"recovery_code_hashes,omitempty"`
	Passkeys             []PasskeyCredential `json:"passkeys,omitempty"`
}

type ClusterConfig struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	UseSSL         bool   `json:"use_ssl"`
	ForcePathStyle bool   `json:"force_path_style"`
	AdminEndpoint  string `json:"admin_endpoint"`
	AdminToken     string `json:"admin_token"`
}

type State struct {
	Admin    AdminAccount    `json:"admin"`
	Clusters []ClusterConfig `json:"clusters"`
}

type Manager struct {
	path  string
	state State
	mu    sync.RWMutex
}

func NewManager(path string) (*Manager, error) {
	m := &Manager{path: path}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create state directory %q: %w", dir, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.state = State{
				Clusters: []ClusterConfig{},
			}
			if err := m.Save(); err != nil {
				return nil, err
			}
			return m, nil
		}
		return nil, fmt.Errorf("read state file %q: %w", path, err)
	}

	if err := json.Unmarshal(data, &m.state); err != nil {
		return nil, fmt.Errorf("parse state file %q: %w", path, err)
	}
	if m.state.Admin.Setup && (m.state.Admin.SecurityVersion < CurrentSecurityVersion || len(m.state.Admin.WebAuthnUserHandle) != 64) {
		if err := ensureAdminSecurity(&m.state.Admin); err != nil {
			return nil, err
		}
		if err := m.Save(); err != nil {
			return nil, fmt.Errorf("migrate admin security state: %w", err)
		}
	}

	return m, nil
}

func (m *Manager) Save() error {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()
	return m.save(s)
}

func (m *Manager) save(s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// Write via temp + rename so a crash mid-write cannot leave a truncated file.
	dir := filepath.Dir(m.path)
	tmp, err := os.CreateTemp(dir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if f, err := os.OpenFile(tmpName, os.O_WRONLY, 0600); err == nil {
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return fmt.Errorf("sync temp state file: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close synced state file: %w", err)
		}
	} else {
		return fmt.Errorf("reopen temp state file: %w", err)
	}
	if err := os.Rename(tmpName, m.path); err != nil {
		return fmt.Errorf("replace state file %q: %w", m.path, err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		if err := dirHandle.Sync(); err != nil {
			_ = dirHandle.Close()
			return fmt.Errorf("sync state directory: %w", err)
		}
		if err := dirHandle.Close(); err != nil {
			return fmt.Errorf("close state directory: %w", err)
		}
	} else {
		return fmt.Errorf("open state directory: %w", err)
	}
	cleanup = false
	return nil
}

func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneState(m.state)
}

func cloneState(s State) State {
	data, _ := json.Marshal(s)
	var clone State
	_ = json.Unmarshal(data, &clone)
	return clone
}

func ensureAdminSecurity(admin *AdminAccount) error {
	if !admin.Setup {
		return nil
	}
	if admin.SecurityVersion < CurrentSecurityVersion {
		admin.SecurityVersion = CurrentSecurityVersion
	}
	if len(admin.WebAuthnUserHandle) != 64 {
		admin.WebAuthnUserHandle = make([]byte, 64)
		if _, err := rand.Read(admin.WebAuthnUserHandle); err != nil {
			return fmt.Errorf("generate WebAuthn user handle: %w", err)
		}
	}
	return nil
}

func (m *Manager) UpdateAdmin(admin AdminAccount) error {
	return m.MutateAdmin(func(current *AdminAccount) error { *current = admin; return nil })
}

// MutateAdmin applies and persists one account change while holding the lock.
// The callback receives a deep copy and failed saves never become visible.
func (m *Manager) MutateAdmin(fn func(*AdminAccount) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	candidate := cloneState(m.state)
	if err := fn(&candidate.Admin); err != nil {
		return err
	}
	if err := ensureAdminSecurity(&candidate.Admin); err != nil {
		return err
	}
	if err := m.save(candidate); err != nil {
		return err
	}
	m.state = candidate
	return nil
}

// SetupAdmin atomically claims first-time setup and persists it before making
// the new account visible to other requests.
func (m *Manager) SetupAdmin(admin AdminAccount) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Admin.Setup {
		return false, nil
	}
	if err := ensureAdminSecurity(&admin); err != nil {
		return false, err
	}
	candidate := m.state
	candidate.Admin = admin
	if err := m.save(candidate); err != nil {
		return false, err
	}
	m.state = candidate
	return true, nil
}

func (m *Manager) AddCluster(c ClusterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	candidate := m.state
	candidate.Clusters = append(append([]ClusterConfig(nil), m.state.Clusters...), c)
	if err := m.save(candidate); err != nil {
		return err
	}
	m.state = candidate
	return nil
}

func (m *Manager) RemoveCluster(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newClusters := make([]ClusterConfig, 0, len(m.state.Clusters))
	for _, c := range m.state.Clusters {
		if c.ID != id {
			newClusters = append(newClusters, c)
		}
	}
	candidate := m.state
	candidate.Clusters = newClusters
	if err := m.save(candidate); err != nil {
		return err
	}
	m.state = candidate
	return nil
}

func (m *Manager) UpdateCluster(c ClusterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, existing := range m.state.Clusters {
		if existing.ID == c.ID {
			candidate := m.state
			candidate.Clusters = append([]ClusterConfig(nil), m.state.Clusters...)
			candidate.Clusters[i] = c
			if err := m.save(candidate); err != nil {
				return err
			}
			m.state = candidate
			return nil
		}
	}
	return nil
}

// ValidateClusterEndpoints is the strict default validator: it rejects any
// control-plane target that could reach a local, link-local, metadata, or
// private (RFC 1918 / unique-local) address. See ValidateClusterEndpointsAllowlist
// for a variant that lets self-hosted deployments opt into private ranges.
func ValidateClusterEndpoints(endpoints ...string) error {
	return ValidateClusterEndpointsAllowlist(nil, endpoints...)
}

// ValidateClusterEndpointsAllowlist rejects control-plane targets that could
// reach local, link-local, or cloud metadata services. DNS is resolved before
// persistence so a hostname cannot bypass the IP checks.
//
// In addition to those always-blocked ranges, private address space
// (net.IP.IsPrivate: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 and fc00::/7)
// is rejected unless the operator explicitly lists the IP/CIDR in
// allowlistCIDRs. This keeps the default fail-closed for cloud metadata /
// internal pivoting while permitting legitimate self-hosted clusters that
// deliberately run on a private network.
func ValidateClusterEndpointsAllowlist(allowlistCIDRs []string, endpoints ...string) error {
	allowlist, err := parseIPAllowlist(allowlistCIDRs)
	if err != nil {
		return err
	}
	for _, raw := range endpoints {
		if !strings.Contains(raw, "://") {
			raw = "http://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" {
			return fmt.Errorf("invalid cluster endpoint")
		}
		if u.Port() != "" {
			if p, err := strconv.Atoi(u.Port()); err != nil || p <= 0 || p > 65535 {
				return fmt.Errorf("invalid cluster endpoint")
			}
		}
		ips, err := net.LookupIP(u.Hostname())
		if err != nil || len(ips) == 0 {
			return fmt.Errorf("cluster endpoint host could not be resolved")
		}
		for _, ip := range ips {
			if isBlockedClusterTarget(ip, allowlist) {
				return &BlockedEndpointError{Host: u.Hostname(), IP: ip.String(), Private: ip.IsPrivate()}
			}
		}
	}
	return nil
}

// BlockedEndpointError says which address a cluster endpoint was refused for.
// Private addresses can be opted into through the endpoint allowlist; the
// others (loopback, link-local, metadata) never can.
type BlockedEndpointError struct {
	Host    string
	IP      string
	Private bool
}

func (e *BlockedEndpointError) Error() string {
	if e.Private {
		return fmt.Sprintf("cluster endpoint %s resolves to private address %s, which is not in the cluster endpoint allowlist (Settings → Security)", e.Host, e.IP)
	}
	return fmt.Sprintf("cluster endpoint %s resolves to %s, a loopback, link-local or metadata address, which is never allowed", e.Host, e.IP)
}

// isBlockedClusterTarget reports whether ip is a target the control plane
// should never reach. Loopback, link-local, and cloud metadata addresses are
// always blocked (they cannot be legitimated by configuration). Private ranges
// are blocked only when they do not match an entry in allowlist.
func isBlockedClusterTarget(ip net.IP, allowlist []*net.IPNet) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.String() == "169.254.169.254" {
		return true
	}
	if !ip.IsPrivate() {
		return false // public IP — never a local/metadata target
	}
	for _, cidr := range allowlist {
		if cidr.Contains(ip) {
			return false // operator explicitly opted into this private range
		}
	}
	return true
}

// parseIPAllowlist parses CIDR or bare-IP entries into *net.IPNet. A bare IP is
// treated as a single-host /32 or /128 network. An unparsable entry is a hard
// error so a typo can never silently widen or leak policy.
func parseIPAllowlist(entries []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid cluster endpoint allowlist entry %q: %v", entry, err)
		}
		out = append(out, ipNet)
	}
	return out, nil
}

func (m *Manager) GetCluster(id string) (ClusterConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.state.Clusters {
		if c.ID == id {
			return c, true
		}
	}
	return ClusterConfig{}, false
}
