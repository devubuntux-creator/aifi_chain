package ethapi

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// NewtonAPI offers specialized RPC methods for the AifiNewton AI engine.
type NewtonAPI struct {
	b Backend
}

// NewNewtonAPI creates a new instance of the Newton RPC API.
func NewNewtonAPI(b Backend) *NewtonAPI {
	return &NewtonAPI{b}
}

// PredictGas calculates the expected gas cost for an aiVectorMatch operation.
// This allows developers to estimate costs without sending a trial transaction.
// Method Name: newton_predictGas
func (s *NewtonAPI) PredictGas(ctx context.Context, input hexutil.Bytes) (hexutil.Uint64, error) {
	// 1. Validation: Ensure input follows the [VecA][VecB] 16-byte multiple rule.
	if len(input) < 16 || len(input)%16 != 0 {
		return 0, fmt.Errorf("invalid input length: must be a multiple of 16 bytes")
	}

	// 2. Logic: Mirror the Precompiled Contract's gas formula
	// Base (500) + 10 gas per 16-byte dimension pair
	gas := 500 + uint64(len(input)/16)*10

	return hexutil.Uint64(gas), nil
}

// GetSimilarity (Placeholder for your previously discussed method)
func (s *NewtonAPI) GetSimilarity(ctx context.Context, vecA, vecB hexutil.Bytes) (hexutil.Uint64, error) {
	// Logic to return the 1e18 scaled similarity score
	return 0, nil
}

// NewtonStatus represents the current state of the AI engine on this node.
// NewtonStatus represents the current state and configuration of the AifiNewton AI engine.
type NewtonStatus struct {
	Version       string `json:"version"`       // Engine version (e.g., "1.0.0-stable")
	IsActive      bool   `json:"isActive"`      // True if Newton hardfork is active at current height
	EngineAddress string `json:"engineAddress"` // The hex address of the AI precompiled contract (0xA1)
	MaxDimensions int    `json:"maxDimensions"` // Recommended max dimensions for 0.5s block stability
	ScalingFactor string `json:"scalingFactor"` // The multiplier used for float-to-int conversion (1e18)
	NewtonBlock   uint64 `json:"newtonBlock"`   // The block height where Newton features activate
}

// NodeStatus returns metadata about the AifiNewton AI environment.
// It allows client-side applications to discover the engine's address and
// activation status without hardcoding values in the frontend.
func (s *NewtonAPI) NodeStatus(ctx context.Context) (*NewtonStatus, error) {
	// Retrieve the chain configuration and the latest header from the backend
	config := s.b.ChainConfig()
	currentBlock := s.b.CurrentHeader().Number

	// Determine if the fork is active by comparing current height with NewtonBlock
	isActive := false
	if config.NewtonBlock != nil && currentBlock != nil {
		isActive = currentBlock.Uint64() >= config.NewtonBlock.Uint64()
	}

	// Initialize the status object with AifiNewton protocol constants
	status := &NewtonStatus{
		Version:  "AifiNewton-v1.0-Standard",
		IsActive: isActive,
		// Explicitly using 0xA1 to avoid future collisions with BSC/Ethereum updates
		EngineAddress: "0x00000000000000000000000000000000000000A1",
		MaxDimensions: 2048,    // Hint for developers to avoid extremely large vector computations
		ScalingFactor: "10^18", // Standard scaling for EVM compatibility (matches Ether/Wei precision)
	}

	// Set the activation block if defined in the chain configuration
	if config.NewtonBlock != nil {
		status.NewtonBlock = config.NewtonBlock.Uint64()
	}

	return status, nil
}

// IndicatorRequest defines the parameters for technical indicator calculation.
type IndicatorRequest struct {
	Symbol    string   `json:"symbol"`    // e.g., "AIFI/USDT"
	Indicator string   `json:"indicator"` // e.g., "RSI", "BB", "EMA"
	Period    int      `json:"period"`    // e.g., 14, 20, 200
	Prices    []uint64 `json:"prices"`    // Latest price series (scaled by 1e18)
}

// IndicatorResponse returns the calculated quantitative values.
type IndicatorResponse struct {
	Indicator string            `json:"indicator"`
	Value     string            `json:"value"`    // Primary result (e.g., RSI value)
	Metadata  map[string]string `json:"metadata"` // Additional data (e.g., Upper/Lower bands for BB)
	Timestamp uint64            `json:"timestamp"`
}

// GetMarketIndicator provides high-performance technical analysis indicators.
// It leverages the node's native math engine to provide instant results for AI Agents.
func (s *NewtonAPI) GetMarketIndicator(ctx context.Context, args IndicatorRequest) (*IndicatorResponse, error) {
	if len(args.Prices) < args.Period {
		return nil, fmt.Errorf("insufficient price data: require at least % d points", args.Period)
	}

	// Convert uint64 (1e18 scaled) to float64 for internal calculation
	floatPrices := make([]float64, len(args.Prices))
	for i, p := range args.Prices {
		floatPrices[i] = float64(p) / 1e18
	}

	response := &IndicatorResponse{
		Indicator: args.Indicator,
		Metadata:  make(map[string]string),
		Timestamp: uint64(time.Now().Unix()),
	}

	switch strings.ToUpper(args.Indicator) {
	case "RSI":
		// Relative Strength Index Calculation
		rsi := calculateRSI(floatPrices, args.Period)
		response.Value = fmt.Sprintf("%.18f", rsi)

	case "BB":
		// Bollinger Bands: Returns Middle, Upper, and Lower bands
		middle, upper, lower := calculateBollingerBands(floatPrices, args.Period)
		response.Value = fmt.Sprintf("%.18f", middle) // SMA as primary value
		response.Metadata["upper"] = fmt.Sprintf("%.18f", upper)
		response.Metadata["lower"] = fmt.Sprintf("%.18f", lower)

	case "EMA":
		// Exponential Moving Average
		ema := calculateEMA(floatPrices, args.Period)
		response.Value = fmt.Sprintf("%.18f", ema)

	default:
		return nil, fmt.Errorf("unsupported indicator: %s", args.Indicator)
	}

	return response, nil
}

// calculateBollingerBands returns (Middle, Upper, Lower) bands.
func calculateBollingerBands(prices []float64, period int) (float64, float64, float64) {
	// 1. Calculate SMA (Middle Band)
	var sum float64
	relevantPrices := prices[len(prices)-period:]
	for _, p := range relevantPrices {
		sum += p
	}
	sma := sum / float64(period)

	// 2. Calculate Standard Deviation
	var variance float64
	for _, p := range relevantPrices {
		variance += math.Pow(p-sma, 2)
	}
	stdDev := math.Sqrt(variance / float64(period))

	return sma, sma + (2 * stdDev), sma - (2 * stdDev)
}

// calculateRSI implements the Relative Strength Index.
// It uses the last 'period' elements to determine market momentum.
func calculateRSI(prices []float64, period int) float64 {
	if len(prices) <= period {
		return 50.0 // Neutral if not enough data
	}

	var avgGain, avgLoss float64

	// Initial RSI calculation
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change
		}
	}

	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Smoothed RSI calculation for the rest of the series
	for i := period + 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		var gain, loss float64
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// calculateEMA computes the Exponential Moving Average.
// Formula: EMA = [Price(current) - EMA(prev)] * Multiplier + EMA(prev)
// Multiplier = 2 / (Period + 1)
func calculateEMA(prices []float64, period int) float64 {
	if len(prices) == 0 {
		return 0
	}
	if len(prices) < period {
		period = len(prices)
	}

	multiplier := 2.0 / (float64(period) + 1.0)

	// Start with SMA as the first EMA value for the base
	var sma float64
	firstSlice := prices[:period]
	for _, p := range firstSlice {
		sma += p
	}
	currentEma := sma / float64(period)

	// Iteratively calculate EMA for the remaining prices
	for i := period; i < len(prices); i++ {
		currentEma = (prices[i]-currentEma)*multiplier + currentEma
	}

	return currentEma
}
