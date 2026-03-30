## Distributed File Storage System

## Project Overview
We have developed a distributed file storage system designed for high availability, fault tolerance, and strong consistency. The system supports concurrent read and write operations from multiple clients and ensures that uploaded files are replicated across the cluster to prevent data loss.

### 🚀 Core Engineering Team (Group 8)

| Responsibility | Full Name | Student ID | Academic Email |
| :--- | :--- | :--- | :--- |
| 🛡️ **Fault Tolerance** | Ranasinghe A.B | `IT24103447` | IT24103447@my.sliit.lk |
| 🔄 **Replication & Consistency** | Wijerathna T.M.V | `IT24101662` | IT24101662@my.sliit.lK |
| ⏱️ **Time Synchronization** | Somarathne H.D.P.Y | `IT24101854` | IT24101854@my.sliit.lk |
| 🤝 **Consensus / Raft** | Gunasekara A.S.W | `IT24101656` | IT24101656@my.sliit.lk |

## 🚀 Getting Started (First-Time Setup)

Follow these 4 steps to launch the entire distributed cluster and dashboard.

### Step 1: Verify Prerequisites
Ensure you have the following installed on your Windows machine:
- **Go 1.18+**: [Download here](https://go.dev/dl/)
- **Node.js & NPM**: [Download here](https://nodejs.org/)
- **Terminal**: Use Command Prompt or PowerShell (Administrator recommended).

### Step 2: Install Frontend Dependencies
The React dashboard requires an initial installation of its node modules.
1. Cleanly navigate to the `Frontend` directory.
2. Run the install command:
```bash
cd Frontend
npm install
cd ..
```

### Step 3: Launch the Distributed Cluster
To build the Go binaries and spawn all 5 nodes plus the dashboard, run the master startup script from the project root:
```bash
scripts\start_all.bat
```

### Step 4: Access the Dashboard
- Once the script finishes, the dashboard will automatically open in your default browser at: **`http://localhost:5173`**
- **Active Nodes**: You should see node1, node2, and node3 status as "Alive".
- **Standby Pool**: Node4 and Node5 are in standby and will appear if you simulate a node failure.

---
*Group 8 - Distributed File Storage System (2026)*


