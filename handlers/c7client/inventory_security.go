package c7client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InventorySecurityModule monitors inventory transactions for potential security vulnerabilities
type InventorySecurityModule struct {
	BaseModule
	mu sync.RWMutex

	// Context and handler
	ctx     context.Context
	handler *C7Handler
	session *proxy.Session

	// Transaction tracking
	transactions     []TransactionRecord
	suspiciousEvents []SecurityEvent

	// Configuration
	config SecurityConfig
}

// TransactionRecord represents a single inventory transaction
type TransactionRecord struct {
	Timestamp     time.Time
	TransactionID int32
	Type          packet.InventoryTransactionType
	Actions       []protocol.InventoryAction
	PlayerPos     protocol.Vec3
	RequestID     int32
}

// SecurityEvent represents a potential security vulnerability
type SecurityEvent struct {
	Timestamp   time.Time
	Severity    string // "LOW", "MEDIUM", "HIGH", "CRITICAL"
	Category    string
	Description string
	Evidence    interface{}
	Exploitable bool
}

// SecurityConfig holds configuration for security monitoring
type SecurityConfig struct {
	LogAllTransactions      bool
	DetectRapidTransactions bool
	DetectDuplicateSlots    bool
	DetectInvalidActions    bool
	DetectDesyncPatterns    bool
	TimeWindowMS            int
	MaxTransactionsPerSec   int
}

// NewInventorySecurityModule creates a new inventory security monitoring module
func NewInventorySecurityModule() *InventorySecurityModule {
	return &InventorySecurityModule{
		transactions:     make([]TransactionRecord, 0),
		suspiciousEvents: make([]SecurityEvent, 0),
		config: SecurityConfig{
			LogAllTransactions:      true,
			DetectRapidTransactions: true,
			DetectDuplicateSlots:    true,
			DetectInvalidActions:    true,
			DetectDesyncPatterns:    true,
			TimeWindowMS:            1000,
			MaxTransactionsPerSec:   10,
		},
	}
}

// Name returns the module name
func (m *InventorySecurityModule) Name() string {
	return "Inventory Security Monitor"
}

// Description returns module description
func (m *InventorySecurityModule) Description() string {
	return "Monitors inventory transactions for security vulnerabilities"
}

// Init initializes the module
func (m *InventorySecurityModule) Init(ctx context.Context, handler *C7Handler) error {
	m.ctx = ctx
	m.handler = handler
	m.log("Inventory Security Monitor initialized")
	m.log("⚠️  This module is for security testing only")
	m.log("Configuration:")
	m.log(fmt.Sprintf("  - Log all transactions: %v", m.config.LogAllTransactions))
	m.log(fmt.Sprintf("  - Detect rapid transactions: %v", m.config.DetectRapidTransactions))
	m.log(fmt.Sprintf("  - Detect duplicate slots: %v", m.config.DetectDuplicateSlots))
	m.log(fmt.Sprintf("  - Detect invalid actions: %v", m.config.DetectInvalidActions))
	m.log(fmt.Sprintf("  - Detect desync patterns: %v", m.config.DetectDesyncPatterns))
	return nil
}

// OnSessionStart is called when session starts
func (m *InventorySecurityModule) OnSessionStart(session *proxy.Session) error {
	m.session = session
	return nil
}

// OnConnect is called when connecting to the server
func (m *InventorySecurityModule) OnConnect(session *proxy.Session) error {
	m.log("Connected - Starting security monitoring")
	m.log("Commands: /inventory-report, /security-events, /clear-logs")
	return nil
}

// PacketCallback handles incoming and outgoing packets
func (m *InventorySecurityModule) PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error) {

	// Monitor inventory transactions (client -> server)
	if toServer {
		switch p := pk.(type) {
		case *packet.InventoryTransaction:
			m.handleInventoryTransaction(p)
		case *packet.ItemStackRequest:
			m.handleItemStackRequest(p)
		case *packet.ContainerClose:
			m.handleContainerClose(p)
		}
	}

	// Monitor server responses
	if !toServer {
		switch p := pk.(type) {
		case *packet.InventoryContent:
			m.handleInventoryContent(p)
		case *packet.InventorySlot:
			m.handleInventorySlot(p)
		case *packet.ItemStackResponse:
			m.handleItemStackResponse(p)
		}
	}

	return pk, nil
}

// handleInventoryTransaction processes inventory transactions
func (m *InventorySecurityModule) handleInventoryTransaction(pk *packet.InventoryTransaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := TransactionRecord{
		Timestamp:     time.Now(),
		TransactionID: int32(len(m.transactions)),
		Type:          pk.TransactionType,
		Actions:       pk.Actions,
	}

	if m.config.LogAllTransactions {
		m.log(fmt.Sprintf("📦 Transaction #%d: Type=%d, Actions=%d",
			record.TransactionID, pk.TransactionType, len(pk.Actions)))
	}

	// Analyze transaction
	m.analyzeTransaction(&record, pk)

	// Store transaction
	m.transactions = append(m.transactions, record)

	// Cleanup old transactions
	m.cleanupOldTransactions()
}

// analyzeTransaction checks for potential vulnerabilities
func (m *InventorySecurityModule) analyzeTransaction(record *TransactionRecord, pk *packet.InventoryTransaction) {
	// Check for rapid transactions
	if m.config.DetectRapidTransactions {
		m.detectRapidTransactions()
	}

	// Check for duplicate slot access
	if m.config.DetectDuplicateSlots {
		m.detectDuplicateSlotAccess(pk.Actions)
	}

	// Check for invalid actions
	if m.config.DetectInvalidActions {
		m.detectInvalidActions(pk.Actions)
	}

	// Check for desync patterns
	if m.config.DetectDesyncPatterns {
		m.detectDesyncPatterns(pk)
	}
}

// detectRapidTransactions checks for abnormally fast transaction rates
func (m *InventorySecurityModule) detectRapidTransactions() {
	now := time.Now()
	windowStart := now.Add(-time.Duration(m.config.TimeWindowMS) * time.Millisecond)

	count := 0
	for i := len(m.transactions) - 1; i >= 0; i-- {
		if m.transactions[i].Timestamp.Before(windowStart) {
			break
		}
		count++
	}

	if count > m.config.MaxTransactionsPerSec {
		m.logSecurityEvent(SecurityEvent{
			Timestamp:   now,
			Severity:    "MEDIUM",
			Category:    "Rapid Transactions",
			Description: fmt.Sprintf("Detected %d transactions in %dms (threshold: %d)", count, m.config.TimeWindowMS, m.config.MaxTransactionsPerSec),
			Evidence: map[string]interface{}{
				"count":      count,
				"windowMs":   m.config.TimeWindowMS,
				"threshold":  m.config.MaxTransactionsPerSec,
				"recentTxns": count,
			},
			Exploitable: true,
		})
	}
}

// detectDuplicateSlotAccess checks if the same slot is accessed multiple times
func (m *InventorySecurityModule) detectDuplicateSlotAccess(actions []protocol.InventoryAction) {
	slotMap := make(map[string]int)

	for _, action := range actions {
		key := fmt.Sprintf("%d:%d", action.SourceType, action.InventorySlot)
		slotMap[key]++

		if slotMap[key] > 1 {
			m.logSecurityEvent(SecurityEvent{
				Timestamp:   time.Now(),
				Severity:    "HIGH",
				Category:    "Duplicate Slot Access",
				Description: fmt.Sprintf("Slot accessed %d times in single transaction (SourceType=%d, Slot=%d)", slotMap[key], action.SourceType, action.InventorySlot),
				Evidence: map[string]interface{}{
					"sourceType": action.SourceType,
					"slot":       action.InventorySlot,
					"count":      slotMap[key],
					"oldItem":    action.OldItem.Stack.ItemType.NetworkID,
					"newItem":    action.NewItem.Stack.ItemType.NetworkID,
				},
				Exploitable: true,
			})
		}
	}
}

// detectInvalidActions checks for logically invalid inventory actions
func (m *InventorySecurityModule) detectInvalidActions(actions []protocol.InventoryAction) {
	for _, action := range actions {
		// Check for item creation (air -> item)
		if action.OldItem.Stack.ItemType.NetworkID == 0 && action.NewItem.Stack.ItemType.NetworkID != 0 {
			if action.SourceType != protocol.InventoryActionSourceCreative &&
				action.SourceType != protocol.InventoryActionSourceCraft {
				m.logSecurityEvent(SecurityEvent{
					Timestamp:   time.Now(),
					Severity:    "CRITICAL",
					Category:    "Item Creation",
					Description: fmt.Sprintf("Item appeared from air (SourceType=%d, Slot=%d)", action.SourceType, action.InventorySlot),
					Evidence: map[string]interface{}{
						"sourceType": action.SourceType,
						"slot":       action.InventorySlot,
						"itemID":     action.NewItem.Stack.ItemType.NetworkID,
						"count":      action.NewItem.Stack.Count,
					},
					Exploitable: true,
				})
			}
		}

		// Check for count increases without source
		if action.OldItem.Stack.ItemType.NetworkID == action.NewItem.Stack.ItemType.NetworkID {
			if action.NewItem.Stack.Count > action.OldItem.Stack.Count {
				countDiff := action.NewItem.Stack.Count - action.OldItem.Stack.Count
				m.logSecurityEvent(SecurityEvent{
					Timestamp:   time.Now(),
					Severity:    "HIGH",
					Category:    "Count Increase",
					Description: fmt.Sprintf("Item count increased by %d without clear source", countDiff),
					Evidence: map[string]interface{}{
						"sourceType": action.SourceType,
						"slot":       action.InventorySlot,
						"oldCount":   action.OldItem.Stack.Count,
						"newCount":   action.NewItem.Stack.Count,
						"itemID":     action.NewItem.Stack.ItemType.NetworkID,
					},
					Exploitable: true,
				})
			}
		}
	}
}

// detectDesyncPatterns checks for patterns that could cause client-server desync
func (m *InventorySecurityModule) detectDesyncPatterns(pk *packet.InventoryTransaction) {
	// Check for conflicting actions in the same transaction
	if len(pk.Actions) >= 2 {
		// Look for patterns like: Take item -> Place same item in same slot
		for i := 0; i < len(pk.Actions)-1; i++ {
			action1 := pk.Actions[i]
			action2 := pk.Actions[i+1]

			// Check if actions target the same slot
			if action1.SourceType == action2.SourceType &&
				action1.InventorySlot == action2.InventorySlot {
				m.logSecurityEvent(SecurityEvent{
					Timestamp:   time.Now(),
					Severity:    "HIGH",
					Category:    "Potential Desync Pattern",
					Description: fmt.Sprintf("Sequential actions on same slot (Type=%d, Slot=%d)", action1.SourceType, action1.InventorySlot),
					Evidence: map[string]interface{}{
						"sourceType":      action1.SourceType,
						"slot":            action1.InventorySlot,
						"action1OldItem":  action1.OldItem.Stack.ItemType.NetworkID,
						"action1NewItem":  action1.NewItem.Stack.ItemType.NetworkID,
						"action2OldItem":  action2.OldItem.Stack.ItemType.NetworkID,
						"action2NewItem":  action2.NewItem.Stack.ItemType.NetworkID,
						"transactionType": pk.TransactionType,
					},
					Exploitable: true,
				})
			}
		}
	}
}

// handleItemStackRequest processes item stack requests
func (m *InventorySecurityModule) handleItemStackRequest(pk *packet.ItemStackRequest) {
	if m.config.LogAllTransactions {
		m.log(fmt.Sprintf("📋 ItemStackRequest: RequestID=%d, Actions=%d", pk.RequestID, len(pk.Requests)))
	}
}

// handleContainerClose processes container close events
func (m *InventorySecurityModule) handleContainerClose(pk *packet.ContainerClose) {
	m.log(fmt.Sprintf("📪 Container closed: WindowID=%d", pk.WindowID))
}

// handleInventoryContent processes full inventory updates from server
func (m *InventorySecurityModule) handleInventoryContent(pk *packet.InventoryContent) {
	// Server sending full inventory - potential sync response
	if m.config.LogAllTransactions {
		m.log(fmt.Sprintf("📥 Server sent full inventory: WindowID=%d, Items=%d", pk.WindowID, len(pk.Content)))
	}
}

// handleInventorySlot processes single slot updates from server
func (m *InventorySecurityModule) handleInventorySlot(pk *packet.InventorySlot) {
	// Server correcting a slot - possible desync detection
	if m.config.LogAllTransactions {
		m.log(fmt.Sprintf("📥 Server updated slot: WindowID=%d, Slot=%d", pk.WindowID, pk.Slot))
	}
}

// handleItemStackResponse processes server responses to item stack requests
func (m *InventorySecurityModule) handleItemStackResponse(pk *packet.ItemStackResponse) {
	for _, response := range pk.Responses {
		if response.Status != protocol.ItemStackResponseStatusOK {
			m.logSecurityEvent(SecurityEvent{
				Timestamp:   time.Now(),
				Severity:    "MEDIUM",
				Category:    "Server Rejection",
				Description: fmt.Sprintf("Server rejected transaction (RequestID=%d, Status=%d)", response.RequestID, response.Status),
				Evidence: map[string]interface{}{
					"requestID": response.RequestID,
					"status":    response.Status,
				},
				Exploitable: false,
			})
		}
	}
}

// logSecurityEvent records a security event
func (m *InventorySecurityModule) logSecurityEvent(event SecurityEvent) {
	m.suspiciousEvents = append(m.suspiciousEvents, event)

	// Log to console
	exploitableStr := ""
	if event.Exploitable {
		exploitableStr = "⚠️  EXPLOITABLE"
	}
	m.log(fmt.Sprintf("[%s] %s: %s %s", event.Severity, event.Category, event.Description, exploitableStr))
}

// cleanupOldTransactions removes transactions older than 1 minute
func (m *InventorySecurityModule) cleanupOldTransactions() {
	cutoff := time.Now().Add(-1 * time.Minute)
	newTransactions := make([]TransactionRecord, 0)

	for _, txn := range m.transactions {
		if txn.Timestamp.After(cutoff) {
			newTransactions = append(newTransactions, txn)
		}
	}

	m.transactions = newTransactions
}

// HandleCommand processes module commands
func (m *InventorySecurityModule) HandleCommand(cmd string, args []string) bool {
	switch cmd {
	case "inventory-report":
		m.printInventoryReport()
		return true
	case "security-events":
		m.printSecurityEvents()
		return true
	case "clear-logs":
		m.clearLogs()
		return true
	}
	return false
}

// printInventoryReport prints a summary of monitored transactions
func (m *InventorySecurityModule) printInventoryReport() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.log("=== Inventory Security Report ===")
	m.log(fmt.Sprintf("Total transactions monitored: %d", len(m.transactions)))
	m.log(fmt.Sprintf("Security events detected: %d", len(m.suspiciousEvents)))

	// Count by severity
	severityCounts := make(map[string]int)
	for _, event := range m.suspiciousEvents {
		severityCounts[event.Severity]++
	}

	m.log("\nEvents by severity:")
	for severity, count := range severityCounts {
		m.log(fmt.Sprintf("  %s: %d", severity, count))
	}

	// Count exploitable events
	exploitableCount := 0
	for _, event := range m.suspiciousEvents {
		if event.Exploitable {
			exploitableCount++
		}
	}
	m.log(fmt.Sprintf("\nExploitable vulnerabilities: %d", exploitableCount))
}

// printSecurityEvents prints all security events
func (m *InventorySecurityModule) printSecurityEvents() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.log("=== Security Events ===")
	if len(m.suspiciousEvents) == 0 {
		m.log("No security events detected")
		return
	}

	for i, event := range m.suspiciousEvents {
		exploitable := ""
		if event.Exploitable {
			exploitable = " [EXPLOITABLE]"
		}
		m.log(fmt.Sprintf("\n#%d [%s] %s%s", i+1, event.Severity, event.Category, exploitable))
		m.log(fmt.Sprintf("  Time: %s", event.Timestamp.Format("15:04:05")))
		m.log(fmt.Sprintf("  Description: %s", event.Description))
		if event.Evidence != nil {
			m.log(fmt.Sprintf("  Evidence: %+v", event.Evidence))
		}
	}
}

// clearLogs clears all stored transactions and events
func (m *InventorySecurityModule) clearLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transactions = make([]TransactionRecord, 0)
	m.suspiciousEvents = make([]SecurityEvent, 0)
	m.log("Logs cleared")
}

// OnSessionEnd is called when the session ends
func (m *InventorySecurityModule) OnSessionEnd(session *proxy.Session) {
	m.log("Session ended - Generating final report")
	m.printInventoryReport()
}

// Cleanup cleans up module resources
func (m *InventorySecurityModule) Cleanup() {
	m.log("Cleanup complete")
}

// log outputs a message to the console
func (m *InventorySecurityModule) log(message string) {
	fmt.Printf("[Inventory Security] %s\n", message)
}
