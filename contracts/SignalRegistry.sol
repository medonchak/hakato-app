// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Records every agent trading decision before execution.
/// Full audit trail: token, signal type, reason, confidence score.
contract SignalRegistry {
    address public owner;

    enum SignalType { NONE, BUY, SELL, HOLD }

    struct Signal {
        uint64     chainId;
        address    token;
        string     tokenSymbol;
        SignalType signalType;
        string     reason;
        uint32     confidence;   // 0-10000 (basis points, e.g. 7500 = 75%)
        uint32     vwapPeriod;   // hours
        int32      buyThresholdBps;   // basis points below VWAP to buy
        int32      sellThresholdBps;  // basis points above VWAP to sell
        uint256    priceUsd;     // scaled ×1e8
        uint64     timestamp;
        bool       executed;
    }

    struct ActiveStrategy {
        uint32  vwapPeriod;
        int32   buyThresholdBps;
        int32   sellThresholdBps;
        uint32  sharpe;       // scaled ×100
        uint64  updatedAt;
    }

    Signal[] public signals;

    // token address => active strategy
    mapping(address => ActiveStrategy) public strategies;

    event SignalRecorded(
        uint256 indexed id,
        address indexed token,
        SignalType signalType,
        uint32  confidence,
        uint64  timestamp
    );

    event StrategyUpdated(
        address indexed token,
        uint32  vwapPeriod,
        int32   buyThresholdBps,
        int32   sellThresholdBps,
        uint32  sharpe
    );

    event SignalExecuted(uint256 indexed id);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    function recordSignal(
        uint64     chainId,
        address    token,
        string     calldata tokenSymbol,
        SignalType signalType,
        string     calldata reason,
        uint32     confidence,
        uint32     vwapPeriod,
        int32      buyThresholdBps,
        int32      sellThresholdBps,
        uint256    priceUsd
    ) external onlyOwner returns (uint256 id) {
        id = signals.length;
        signals.push(Signal({
            chainId:          chainId,
            token:            token,
            tokenSymbol:      tokenSymbol,
            signalType:       signalType,
            reason:           reason,
            confidence:       confidence,
            vwapPeriod:       vwapPeriod,
            buyThresholdBps:  buyThresholdBps,
            sellThresholdBps: sellThresholdBps,
            priceUsd:         priceUsd,
            timestamp:        uint64(block.timestamp),
            executed:         false
        }));
        emit SignalRecorded(id, token, signalType, confidence, uint64(block.timestamp));
    }

    function markExecuted(uint256 id) external onlyOwner {
        require(id < signals.length, "bad id");
        signals[id].executed = true;
        emit SignalExecuted(id);
    }

    function updateStrategy(
        address token,
        uint32  vwapPeriod,
        int32   buyThresholdBps,
        int32   sellThresholdBps,
        uint32  sharpe
    ) external onlyOwner {
        strategies[token] = ActiveStrategy({
            vwapPeriod:       vwapPeriod,
            buyThresholdBps:  buyThresholdBps,
            sellThresholdBps: sellThresholdBps,
            sharpe:           sharpe,
            updatedAt:        uint64(block.timestamp)
        });
        emit StrategyUpdated(token, vwapPeriod, buyThresholdBps, sellThresholdBps, sharpe);
    }

    function totalSignals() external view returns (uint256) {
        return signals.length;
    }

    function getSignal(uint256 id) external view returns (Signal memory) {
        return signals[id];
    }

    function getStrategy(address token) external view returns (ActiveStrategy memory) {
        return strategies[token];
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero address");
        owner = newOwner;
    }
}
