// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Logs every anomaly alert on-chain for verifiable audit trail.
contract AnomalyLogger {
    address public owner;

    struct AnomalyEvent {
        uint64  chainId;
        address token;
        string  reason;
        uint32  severity;   // scaled ×100, e.g. 125 = 1.25
        uint64  hourTs;
        string  txHash;     // originating tx (off-chain reference)
        uint64  timestamp;
    }

    AnomalyEvent[] public events;

    event AnomalyLogged(
        uint256 indexed id,
        uint64  indexed chainId,
        address indexed token,
        string  reason,
        uint32  severity,
        uint64  timestamp
    );

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    function logAnomaly(
        uint64  chainId,
        address token,
        string  calldata reason,
        uint32  severity,
        uint64  hourTs,
        string  calldata txHash
    ) external onlyOwner returns (uint256 id) {
        id = events.length;
        events.push(AnomalyEvent({
            chainId:   chainId,
            token:     token,
            reason:    reason,
            severity:  severity,
            hourTs:    hourTs,
            txHash:    txHash,
            timestamp: uint64(block.timestamp)
        }));
        emit AnomalyLogged(id, chainId, token, reason, severity, uint64(block.timestamp));
    }

    function totalEvents() external view returns (uint256) {
        return events.length;
    }

    function getEvent(uint256 id) external view returns (AnomalyEvent memory) {
        return events[id];
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero address");
        owner = newOwner;
    }
}
