# NucleusMem: A Distributed Memory Layer for AI Agents

## 1、Introduction

The purpose of building this system is to provide resilient memory and communication services for **large-scale AI systems**.At the outset of project design, to enable more flexible memory modifications, development began with a distributed database kernel. However, due to the kernel's complexity and insufficient stability—which led to issues like phantom reads—it will be replaced with an industry-standard stable kernel engine (Individual developers who are not database kernel development specialists simply lack the resources to undertake kernel-level work QAQ) . The prototype currently uses TinyKV but will transition to TiKV in subsequent phases. Future documentation will ignore the details regarding this project's database component, and the project will undergo refactoring to adapt to cloud-native environments.you can see [NucleusMem](https://github.com/cheng-zhangpei/NucleusMem).You can check out this repository to follow future developments.

## 1.1 Challenges

### 1.1.1 Topological Rigidity and Lack of Flexibility

Existing mainstream frameworks (such as LangGraph, AutoGen, and CrewAI) are fundamentally focused on application-level orchestration. Their agent interaction topologies are typically defined statically at startup. In long-term operational scenarios, systems lack the ability to dynamically adjust at runtime. For instance, when sudden spikes in system load require automatic scaling of Agent instances, or when a specific expert Agent needs to be hot-plugged in, existing frameworks often necessitate restarting the entire process or hard-coding all possibilities in advance. This approach fails to adapt to the dynamic scaling demands of cloud environments.

### 1.1.2 State Coupling and “Statelessness”

The core principle of cloud-native architecture is the “statelessness” of compute nodes, yet the essence of agents is “strongly stateful” (dependent on context and memory).

- **Pain point**: Most frameworks tightly bind memory lifecycles to agent processes (in-process memory).
  - Disaster Recovery Challenges: Unpersisted memory is immediately lost upon agent container failure or rescheduling.
  - Data Silos: Agent B cannot access Agent A's experience in real-time, requiring explicit message passing that hinders collaborative efficiency.
  - Inheritance Gap: Newly launched agents cannot directly mount historical “experience slices” accumulated by previous agents, preventing knowledge accumulation within the system.

### 1.1.3 The Vacuum of Collective Memory

Existing memory middleware (e.g., Mem0, Zep) is primarily designed to enhance the interaction experience between individual agents and users (User-Agent Interaction).

- **Pain Points**: They lack foundational primitives for agent swarms. They cannot handle high-concurrency read/write operations and consistency synchronization across multiple agents accessing the same memory space, making it difficult to support efficient swarm collaboration based on the “blackboard pattern” (where multiple agents share the same memory content).

## 1.2 Core Design Principles

### 1.2.1 Memory as Service

This system decouples agent memory from the reasoning process of large models, abstracting the agent's memory into a MemSpace. Large models are inherently stateless black boxes whose state depends entirely on external token inputs. MemSpace fully handles the storage of the agent's memory and state, enabling agents to freely attach and detach MemSpaces to inherit different states.

**Conceptually, a MemSpace is akin to a Persistent Volume (PV) in Kubernetes, while an Agent is a stateless Pod.** This separation brings 2 critical advantages to agent orchestration:

1. **Dynamic State Binding & Context Switching**

In traditional frameworks, an agent's memory is often bound to its process lifecycle. If the process dies, the memory is lost. In NucleusMem, an agent can dynamically **mount** a MemSpace at runtime.

- **Context Switching**: A single agent runtime can switch between different personas (e.g., "Developer" vs. "Auditor") simply by unmounting one MemSpace and mounting another.
- **State Inheritance**: When an agent scales up or migrates to a new node, it instantly inherits the complete historical context by attaching to the existing MemSpace, ensuring zero information loss.

2. **Hierarchical Memory Isolation (Private vs. Shared)**

MemSpace is not a flat storage bucket; it provides a structured isolation mechanism to ensure data privacy and collaboration efficiency.

- **Private MemSpace**: Dedicated to a single agent for storing internal thought chains (CoT), draft buffers, and sensitive credentials. It is strictly isolated from other agents to prevent privacy leakage.
- **Shared MemSpace**: Designed for collaboration. Multiple agents can mount the same Shared MemSpace simultaneously. It acts as a **"Blackboard"**, where Agent A writes a task, and Agent B reads and executes it. This shared state is synchronized across distributed nodes via the underlying consensus protocol.

In summary, **Memory as Service** transforms agent memory from an ephemeral, process-local variable into a **persistent, shareable, and programmable infrastructure resource**. This is the foundation for building resilient and scalable Multi-Agent Systems.

> If the memory of the llm can be a kind of cloud service?

### 1.2.2 Memory as Communication

Traditional multi-agent systems typically rely on explicit peer-to-peer (P2P) messaging, which results in complex and fragile communication links. Introducing a dedicated network layer component to manage agent states within such a system would lead to a highly fragmented architecture, significantly increasing operational complexity and difficulty.

Therefore, NucleusMem integrates the communication process directly into the **MemSpace** component, adopting a **"Store-and-Forward"** paradigm similar to a Blackboard architecture. Communication is no longer a transient network packet but a persistent state change. This approach naturally enables auditing, tracing, and time-travel debugging. To realize this vision, NucleusMem addresses three key challenges:

**1. Message Flow Process: The Watcher Mechanism**

Unlike traditional message queues that push data blindly, NucleusMem utilizes a storage-centric event loop.

- **Write-Driven Trigger**: When Agent A sends a message, it essentially performs a `Put` operation into a specific **Topic Area** (implemented via Key Prefixes) within the MemSpace.
- **Reactive Dispatch**: A lightweight component called **Watcher** monitors these key changes. Upon detecting a new entry, the Watcher retrieves the content and pushes the signal (e.g., the Message Key) to the subscribers' in-memory channels.
- **Pull-Based Retrieval**: The receiving Agent B, upon getting the notification, pulls the full message content from the storage layer. This separation of "Signal Notification" and "Data Retrieval" reduces memory pressure on the routing layer and ensures data consistency.

**2. Routing Determined by the Agent**

Routing decisions are offloaded from the infrastructure to the agents themselves, allowing for flexible topology definitions.

- **Topic as Channel**: Agents communicate by subscribing to specific **Topics** (e.g., "general", "task-1"). A Topic acts as a logical channel mapped to a specific key range in the underlying KV store.
- **Directed Addressing**: For P2P communication, an agent specifies a `TargetID`. The Watcher uses an in-memory routing table to map the ID to the specific channel of the destination agent instance.
- **Semantic Routing (Future Work)**: The architecture reserves the potential for LLM-based routing, where a "Router Agent" analyzes the semantic intent of a message and dynamically determines the most appropriate recipient Topic.

> I know this design wouldn't fly in industrial systems, and such uncertainty is hard to avoid in AI systems. But as a personal project, why not let it be a bit radical?

**3. Communication Traceability and Audit**

Since every communication action is fundamentally a storage transaction, NucleusMem provides native auditability without additional overhead.

- **Persistence**: All chat histories are persisted as KV pairs (e.g., `Topic/Timestamp/SenderID`). This allows the system to reconstruct the entire conversation flow even after a cluster restart.
- **Time-Travel Debugging**: Utilizing the underlying MVCC (Multi-Version Concurrency Control) mechanism, developers can query the state of the communication channel at any specific timestamp `T`. This is critical for diagnosing "hallucinations" or logical errors in multi-agent collaborations, as it allows replaying the exact inputs an agent received at a past moment.

## 2. Design

### 2.1 project structure and architecture

```
.
├── api/                    # [Added] Contains .proto and K8s CRD definitions
│   ├── v1/                 # Protobuf (RPC Protocol)
│   └── crd/                # K8s CRD (YAML Definitions)
├── cmd/                    # Entry points (Main functions)
│   ├── standalone/         # Standalone debug entry (Agent + MemSpace + TinyKV all-in-one)
│   ├── compute-node/       # [Data Plane] Agent Runtime image entry
│   └── controller-manager/ # [Control Plane] NucleusMem controller (Non-K8s Operator)
├── pkg/
│   ├── client/             # Go SDK (For user code consumption)
│   ├── controller/         # Business logic control (AgentManager, MemSpaceManager)
│   ├── network/            # Network encapsulation (gRPC, Transport)
│   ├── runtime/            # Runtime logic (Agent Loop, LLM calls)
│   └── storage/            # Storage layer (TinyKV Client, Txn)
├── tinykv/                 # [Submodule] TinyKV source code
└── Dockerfile              # For building container images
```
the system have 2 essential components, controllers and resources.
- controllers: AgentManager(control the agents) and MemSpaceController(control the memSpaces)
- resource:Agent and MemSpace

The job of the Agent is to get the input of the user,and preprocess the input, agent would read the message provided by the memSpace it bind
and generate. To support this lifecycle effectively in a distributed environment, the architecture adopts a **Manager-Monitor** pattern across the cluster.

#### 2.1.1 Deployment Topology: Manager-Monitor Pattern
To manage resources across a distributed cluster (before fully transitioning to Cloud-Native Operators), the system employs a two-tier hierarchy:


*   **Cluster Level (The Managers):**
*   **AgentManager & MemSpaceManager:** Deployed on master nodes. They serve as the central brain, maintaining global metadata, routing tables, and resource allocation strategies.
*   **Role:** When an Agent needs to access memory, it requests the MemSpaceManager to "mount" a specific MemSpace. The manager updates the routing info, allowing the Agent to establish a direct gRPC link to the target storage.

*   **Node Level (The Monitors):**
*   **AgentMonitor & MemSpaceMonitor:** Deployed as daemons on every compute node.
*   **Role:** They act as local executives. They receive instructions from the central Managers (e.g., "Start Agent A", "Stop MemSpace B") and report local resource status and heartbeats back to the cluster.

![cluster_arch](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/distributed_arch.png)

#### 2.1.2 Concurrency Model: Worker Mechanism

In a massive multi-agent system, a naive one-thread-per-request model leads to **"Thread Explosion."** If every Agent or MemSpace connection spawned dedicated threads for heavy operations (like vector embedding or complex IO), the system would suffer from severe context switching overhead.

To solve this, I adopt an asynchronous **Worker Mechanism**, inspired by the *Coprocessor* concept in TiKV and the *RaftStore* threading model:

*   **Compute Offloading:** Instead of processing locally in the main loop, Agents and MemSpaces offload compute-intensive tasks (e.g., Vector Calculations via cloud APIs) and IO-intensive tasks (e.g., generating memory summaries) to a centralized **Worker Pool** managed by the Monitor.
*   **Multiplexing:** A small, fixed number of Worker threads service a large number of Agents. This "Few-to-Many" mapping prevents resource inflation and ensures stable latency even when the Agent count scales up.

![mem_node](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/mem_node.png)
#### 2.1.3 Logic Abstraction: MemSpace Zones & ViewSpace

MemSpace is not just a key-value store; it is a structured environment designed for agent collaboration.

**1. MemSpace Zoning**
Inside a single MemSpace, data is strictly partitioned into four functional zones:
*   **Memory Region:** Stores raw historical data and context.
*   **Task Region:** Stores the Task Chain, workflow status, and step breakdowns.
*   **Summary Region:** Stores compressed snapshots of history ("Snapshots"). These are asynchronously generated by the **Workers** to keep context windows manageable.
*   **Communication Region:** A "Blackboard" area where Agents publish messages and subscribe to topics for real-time collaboration.

**2. ViewSpace (The Task View)**
To organize multi-agent collaboration, we introduce **ViewSpace**. A ViewSpace is a logical communication scope created by connecting to a Public MemSpace.

*   **Represented Agent:** For each major task, a lead agent (Represented Agent) is assigned. It acts as the task owner, initializing the **Global Task View** in the Task Zone and generating **Task Sub-Views** for classification.
*   **Global Trace:** All Agents participating in the task operate within this ViewSpace. Every action and message is "traced" (logged) into the Global MemSpace. This ensures that the entire collaborative process is fully auditable and replayable, providing a unified view of the swarm's progress.

![viewSpance](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/viewSpace.png)

### 2.2 deploy(todo)

- components

| entry (cmd/)       | Binary        | K8s         | replicas   | Role                                                         | dependencies     |
| ------------------ | ------------- | ----------- | ---------- | ------------------------------------------------------------ | ---------------- |
| tinykv-server      | tinykv        | StatefulSet | 3+         | Storage layer. Each Pod has an independent PVC (hard disk) and a fixed network identifier. | PD               |
| pd-server          | pd            | Deployment  | 3          | Scheduling layer. Raft election, stateless (data stored in Etcd). | -                |
| controller-manager | nucleus-ctrl  | Deployment  | 1 (Leader) | Control plane. The logic for the Operator runs here (or independently). | K8s API & PD     |
| compute-node       | nucleus-agent | Deployment  | N          | Computing layer. Runs agent logic, stateless, and is launched on demand. | TinyKV           |
| memspace           | nucleus-mem   | StatefulSet | N          | Memory space layer. Manages persistent memory and state for stateless agents. Each Pod has associated PVC for storing agent memories and states. | K8s API & TinyKV |

- CRD

| Component/Object          | K8s Resource Type | CRD Definition (Key Fields)                                  | Operator Reconcile Logic                                     | Backend Action                                               |
| ------------------------- | ----------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| Cluster (TinyKV+PD)       | NucleusCluster    | Spec: pdReplicas: 3 storeReplicas: 3 image: "v1.0"  Status: ReadyStores: [1, 2, 3] | 1. Check if PD Deployment exists? If not, create it. 2. Check if TinyKV StatefulSet exists? If not, create it. 3. Check if Store count meets storeReplicas requirement? Scale up/down if not. | Launch Pods, Mount PVC, Start processes.                     |
| MemSpace (Memory Space)   | MemSpace          | Spec: name: "shared-01" type: "SHARED" quota: "10Gi"  Status: regionID: 100 peers: [1, 2, 3] | 1. Check if Status.RegionID is empty? 2. If empty -> Call PD Client to request new Region. 3. Update Status to record RegionID. 4. Check if Region replica count meets configuration? Call PD scheduler if not. | Call pd.AllocID, Call Split, Call AddPeer.                   |
| Agent (Intelligent Agent) | Agent             | Spec: role: "Coder" mount: ["shared-01"]  Status: phase: "Running" | 1. Check if Deployment named agent-{name} exists? 2. If not -> Create Deployment. 3. Inject configuration -> Convert MemSpace ID from mount to environment variable MEM_ID=100 and inject into container. 4. Update Status. | Start compute-node process, initialize ClusterClient inside process to connect. |

- phase:

component dev (current) -->  docker compose --> operator dev

### 2.2 the client of tinyKV

As a teaching project under PingCap's talent program, TinyKV does not provide a reliable, complete client (not even a third-party repository on GitHub...). If we aim to use TinyKV as our foundation, we must implement a highly available client ourselves. Within the Percolator transaction model, the client serves as the transaction coordinator with cross-cluster capabilities. Developing a fully production-ready client is challenging, so we've implemented a basic version that handles cross-cluster coordination and backoff retries. This enables NucleusMem to support cross-cluster transactions.

### 2.2.1 architecture

![client](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/arch-of-client.png)

MemClient maintains a Txn retry loop. If the server returns a repeatable error, it triggers the loop's retransmission and backoff mechanism. The backoff mechanism has not yet been designed. Each MemClient maintains an instance of a transaction factory, which generates Txn objects and assigns them initial Txn properties.

Txn maintains two simple buffers to store write operations for the current transaction. During delete and put operations, data is simply saved into the buffer. The Core of Txn is preWrite and commit, will introduce later.

Each transaction maintains a ClusterRouter responsible for managing the region cache, distributing different keys to their corresponding regions, and maintaining the gRPC connection pool.

### 2.2.2 preWrite and commit

The transaction commitment process in `NucleusMem` strictly follows the **Percolator** Two-Phase Commit (2PC) protocol. Since TinyKV servers are stateless regarding transaction coordination, the client (`Txn` object) orchestrates the entire lifecycle.

- Phase 1: Prewrite (Locking)

When the user calls `txn.Commit()`, the client first enters the Prewrite phase. The goal is to lock all keys involved in the transaction and write the data to the storage.

1. **Batch Grouping**: The client iterates through the local write buffer (`puts` and `deletes`). Using the `ClusterRouter`, it groups mutations by their target **Region**. This is crucial because a `KvPrewrite` RPC can only be sent to the Leader of a specific Region.
2. **Primary Key Selection**: The client selects one key as the **Primary Key** (currently, the first key in the buffer). This key's status determines the fate of the entire transaction.(For now, I just maintain the first key of the transaction as the primary key.)
3. Parallel Execution: The client sends KvPrewrite requests to all relevant Region Leaders.
- **Conflict Check**: If the server returns a `WriteConflict` (meaning another transaction committed after our StartTS), the client aborts immediately.
- **Lock Resolution**: If the server returns `KeyLocked`, the client attempts to resolve the lock (querying the status of the lock's Primary Key via `KvCheckTxnStatus` and cleaning it up via `KvResolveLock`) before retrying.
4. **Rollback**: If any Region fails to prewrite (e.g., due to unresolvable conflict or network partition), the client initiates a `Rollback` operation for all keys to clean up partial locks.

- Phase 2: Commit (Making it Visible)

Once all Prewrites succeed, the transaction is considered "locked". The client then proceeds to make the changes visible.

1. Commit Primary: The client requests a CommitTSfrom the TSO (Timestamp Oracle,But now the project just use localTimeStamp). It then sends a KvCommit request only for the Primary Key.
- **Point of No Return**: If the Primary Key is successfully committed, the transaction is logically successful. Even if the client crashes afterwards, other transactions can roll forward the secondary keys by checking the Primary's status.
2. Commit Secondaries (Async): After the Primary commit succeeds, the client sendsKvCommitrequests for all remaining keys (Secondary Keys).
- **Optimization**: This step is performed asynchronously. Since the transaction is already logically committed, failures here are non-fatal (locks will be resolved lazily by future readers).

This client-side coordination ensures **Snapshot Isolation (SI)** and **Atomicity** across multiple distributed nodes, providing a robust consistency guarantee for the upper-level Agent Memory system.

### 2.2.3 dispatch

To support a distributed architecture where keys are sharded across multiple Regions and Stores, the client must intelligently route requests to the correct destination. The `ClusterClient` handles this complexity through a robust dispatch mechanism.

1. Region Location & Caching

The client maintains a local **Region Cache** to minimize interactions with the PD (Placement Driver).

- **Locate**: When a request for a `Key` arrives, the client checks the cache to find the corresponding Region and its Leader's Store address.
- **Cache Miss**: If the cache is empty or stale, the client queries PD to fetch the latest routing information (Region Epoch, Peers, Leader) and updates the local cache.

2. Request Dispatching

The `SendRequest` method serves as the unified entry point for all RPCs (`Get`, `Scan`, `Prewrite`, `Commit`).

- **Context Injection**: It automatically injects the `RegionContext` (RegionID, Epoch, Peer) into the gRPC request header, ensuring the server can validate the request.
- **Protocol Adaptation**: It handles different types of requests using a type switch, invoking the corresponding gRPC method on the target Store's client stub.

3. Error Handling & Retry Loop

The dispatch logic includes a built-in retry loop to handle dynamic cluster changes:

- **NotLeader**: If the server responds that it is not the Leader, the client updates its cache with the Leader hint provided by the server and retries immediately.
- **EpochNotMatch**: If the Region has split or merged, the client invalidates the dirty cache entry and fetches the new topology from PD before retrying.
- **Network Failure**: If the connection fails, the client attempts to reconnect or re-resolve the Store address.

This design decouples the transaction logic from network topology, allowing `Txn` to operate as if it were talking to a single local database.

### 2.2.4 RegionCache

The `RegionCache` is a critical component for performance, acting as a local map of the distributed cluster. Instead of querying PD for every request, the client consults this cache to find the target Region.

#### 1. B-Tree Indexing Strategy

The cache stores Region metadata in a **B-Tree** (using `google/btree`) to support efficient range queries.

- **Key Selection**: The B-Tree indexes Regions by their **EndKey**.

- Why EndKey?: A Region covers a range[StartKey, EndKey). When we search for a specificKey, we need to find the Region that contains it. Since ranges are continuous and non-overlapping, the target Region is the

  first Region whose EndKey is greater than the target Key.

  - *Search Logic*: `Seek(Key)` on the B-Tree returns the smallest item `>= Key`. If we indexed by StartKey, a simple Seek might land us on the *next* Region, not the *containing* Region. Indexing by EndKey allows us to locate the containing Region in `O(log N)` time.

#### 2. Cache Maintenance

The cache is not static; it evolves with the cluster state.

- **Lazy Loading**: The cache starts empty. Regions are loaded from PD only when accessed (Demand-Driven).
- **Invalidation**: When a `RegionError` (e.g., `EpochNotMatch`) occurs during a request, it indicates the cached topology is outdated (e.g., due to a split). The client removes the invalid Region from the B-Tree.
- **Update**: Upon receiving new information (from PD or Leader hints), the client inserts the new Region info. The B-Tree structure automatically handles the ordering, ensuring that subsequent lookups route correctly to the new layout (e.g., finding the correct sub-region after a split).

This mechanism ensures that the client's view of the cluster eventually converges with the actual topology, maintaining high availability even during rebalancing or failover events.

### 2.3 MemSpace

#### 2.3.1 Collaborative Model

MemSpace's collaboration model is abstracted as a workflow driven by Communication Regions, with four functional zones working in mutual coupling.

In multi-agent collaboration scenarios (e.g., Agent 1 needs to notify Agent 2 to perform a task), the interaction process proceeds as follows:

1. **Data Write**: Agent 1 first writes the specific task content or context into the Memory Region.
2. **Signal Trigger**: Agent 1 writes the index (Key) of this content to the Communication Region.
3. **Notification Dispatch**: The Watcher in the Communication Region detects a write event, locates the registry based on the target Agent ID, and pushes the Key to Agent 2.
4. **Data Read**: Upon receiving the Key, Agent 2 reads the specific content from the Memory Region and begins processing.

```
Agent 1                    MemSpace                         Agent 2
  │                    ┌──────────────┐                       │
  │── Write ────────→  │ Memory Region│←── read memory  ────  │
  │                    ├──────────────┤                       │
  │── writ Key ─────→  │  Comm Region │←─ Watcher notify ──→  │
  │                    ├──────────────┤                       │
  │                    │Summary Region│ ←── read Summary ──── │
  │                    └──────────────┘                       │
```

Considering implementation simplicity and data consistency, MemSpace assigns an independent mutex lock to each Region. Since the operation frequency of Agents is primarily constrained by the generation speed of LLMs (typically on the order of seconds), this coarse-grained locking mechanism is entirely sufficient at the current system scale, with lock contention being virtually negligible.

**1. Memory Region**

The Memory Region serves as the primary storage area within MemSpace, responsible for persisting all raw memory content generated by Agents during collaboration.

All read and write operations by Agents on shared memory directly target the Memory Region. When writing, Agents generate keys with a prefix structure (e.g., memory/{agentID}/{type}/{seq}) to enable subsequent targeted retrieval and filtering by source. When reading, Agents locate specific memory entries via key references received from the Comm Region.

The Memory Region also serves as the data source for the Summary Region—when accumulated records in the Memory Region meet certain conditions, the Summary Worker reads historical data from it to generate compressed summaries.

**2. Comm Region**

Comm Region handles two functions: first, retrieving Agent addresses and notifying Agents of their location information; second, persisting communication records.

**Address Table**: Comm Region maintains an Agent address registry. Upon connecting to the Public MemSpace, an Agent calls the registration function to write its ID, channel address, and role description into the registry. Registration supports both autonomous Agent registration and manual registration, with the system providing a unified registration function. Other Agents can discover communicable objects within the current MemSpace through the address table.

**Watcher and Communication Flow**: Comm Region incorporates a built-in Watcher component. The Watcher maintains a receiving channel through which all communication requests from sending Agents first enter. Upon receiving a request, the Watcher executes the following steps:

1. Write message indexes to the Comm Region for persistence. Recorded content includes sender ID, receiver ID, referenced key, reference type (pointing to Memory Region or Summary Region), and timestamp.
2. Query the Agent address registry to obtain the channel address of the target Agent.
3. Push the key reference to the target agent via the client.

There are two communication modes. The first involves an Agent sending newly generated content: the Agent first writes the content to a Memory Region, then passes the corresponding key to the Watcher for forwarding. The second involves an Agent directing other Agents to access existing shared memory or historical summaries: the Agent directly passes an existing key from either the Memory Region or Summary Region to the Watcher for forwarding, without writing new content. The processing logic on the Watcher side is identical for both modes; the only difference lies in which Region the referenced key points to.

**3. Summary Region**

Summary Region stores compressed snapshots of Memory Region to resolve the conflict between the limited context window of Agents and the continuous growth of historical memory.

**Generation Mechanism**: The Summary Region is asynchronously written by an independent Summary Worker, operating outside the primary collaboration path. The Summary Worker is triggered when either the number of records in the Memory Region exceeds a threshold or the time interval since the last summary generation surpasses a configured value. The Worker reads historical data from the Memory Region, generates a compressed summary, and writes it to the Summary Region. The Summary Worker is the sole background Worker within the current MemSpace.

**Passive References**: Agents are not actively notified upon Summary generation. Agents access the Summary Region under two conditions: 1. When an Agent proactively queries for historical context. 2. When directed by another Agent via the Comm Region—where the directing Agent sends a key reference to the Summary Region through a Watcher, prompting the receiving Agent to retrieve the corresponding summary content.

**4. Tool Region**

The Tool Region maintains a registry of external tool definitions and tracks execution history within a MemSpace. It provides a structured interface for Agents to invoke capabilities outside the core memory system.

**Definition Storage:** Tool definitions are persisted as structured schemas containing name, description, parameter specifications, and endpoint configuration. These definitions are stored in the underlying KV store and are shared among all Agents bound to the MemSpace.example:

```go
// MockLintToolDefinition returns the mock_lint tool definition
func MockLintToolDefinition(endpoint string) *configs.ToolDefinition {
	return &configs.ToolDefinition{
		Name:        "mock_lint",
		Description: "A mock linter tool that checks code for violations",
		Tags:        []string{"static-analysis", "code-tools", "test"},
		Parameters: []configs.ToolParam{
			{
				Name:     "path",
				Type:     "string",
				Required: true,
				Default:  "",
			},
			{
				Name:     "config",
				Type:     "string",
				Required: false,
				Default:  ".eslintrc",
			},
		},
		ReturnType: "object",
		Endpoint:   endpoint,
		ExecType:   "http",
		Metadata: map[string]string{
			"version": "1.0.0",
			"author":  "test",
		},
		CreatedAt: 0,
	}
}

```

**Execution Model:** Tool invocations are processed through the Agent's task queue. When a tool call is identified (typically via LLM output parsing), a tool task is created and dispatched to an Executor layer. The Executor abstracts the communication protocol (e.g., HTTP), allowing different tool types to be supported without modifying core Agent logic.

**Execution Records:** Each invocation is recorded with a unique sequence number, capturing input parameters, output results, execution status, and timestamps. This history supports audit requirements and provides context for subsequent operations.

**Agent Integration:** Agents query the Tool Region to discover available capabilities. During conversation processing, tool schemas are provided to the LLM to determine if external action is required. If a tool call is generated, the execution result is returned to the conversation context upon completion.



### 2.4 Agent

#### 2.4.1 Task Queue

At its core, Agent operates on a task queue-driven execution model. All external requests—such as user conversations, collaboration notifications, and more—are encapsulated as tasks and submitted to an internal queue. These tasks are then processed sequentially by background coroutines.

![task_queue](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/task_queue.png)

Each task carries a type identifier (e.g., temp_chat, comm, chat), enabling Agent to invoke distinct processing logic based on the type. This design makes adding new task types (such as tool invocations or scheduled tasks) remarkably straightforward—simply register a new handler.

Although tasks are processed in submission order, strict end-to-end sequencing cannot be guaranteed due to network and external service latency. However, this approximate ordering suffices for most scenarios (e.g., conversations, memory writes).
Agent itself does not store business state. All persistent data is managed through MemSpace, while Agent maintains only temporary conversation context and task scheduling logic.

> The Agent architecture can be extended to support multi-level queue scheduling, ensuring tasks with varying workloads are distributed effectively. However, as this is a demo, we won't elaborate further here.

#### 2.4.2  Recall Queue

To support synchronous request-response patterns over an asynchronous task execution model, Agent implements a **Recall Queue** (Task Result Tracking Mechanism). While the Task Queue handles work distribution, the Recall Queue manages the lifecycle of task results, allowing external callers (e.g., other Agents via `Notify`) to wait for completion without blocking the main processing loop.

**Mechanism** When a task is submitted via `SubmitTask`, a unique **TaskID** is generated (if not already provided). Simultaneously, a result tracking entry is created in an in-memory map (`taskResults`), containing a `Done` channel and placeholders for the result and error. The TaskID is returned to the caller, who can then invoke `GetTaskResult` to wait asynchronously.

**Lifecycle**

1. **Creation:** Upon task submission, a `TaskResult` object is initialized and stored keyed by TaskID.
2. **Completion:** When the background worker finishes processing (e.g., `handleChatTask` completes), it invokes `SetTaskResult`, which writes the response and closes the `Done` channel.
3. **Retrieval:** The caller blocks on the `Done` channel until signaled. Once closed, the result is retrieved, and the tracking entry is cleaned up to prevent memory leaks.
4. **Timeout:** To prevent indefinite hanging (e.g., if a task stalls), `GetTaskResult` enforces a configurable timeout (e.g., 30s). If exceeded, the wait aborts, and an error is returned.

**Task Chaining & ID Inheritance** A critical feature of the Recall Queue is **TaskID inheritance** across task types. For instance, when a `comm` task triggers a subsequent `chat` task (to generate a response), the child task inherits the parent's TaskID. This ensures that the external caller waiting for the `comm` task ultimately receives the final result from the `chat` processing, maintaining end-to-end transparency despite internal task splitting.

**State Management** Similar to the Task Queue, the Recall Queue holds only **transient execution state**. It does not persist business data. All tracking entries are stored in memory and are lost upon Agent restart, which is acceptable as they represent in-flight operations rather than durable business records.

**Extensibility** This design decouples task execution from result retrieval. Future extensions could include:

- **Priority Recall:** High-priority tasks (e.g., emergency alerts) could bypass standard queues but still track results via this mechanism.
- **Batch Recall:** Waiting for multiple task results simultaneously (e.g., `WaitGroup` pattern).
- **Persistent Tracking:** For long-running tasks (hours/days), the tracking state could be offloaded to Redis or a database, though this is beyond the current scope.

By combining the Task Queue (for throughput) and the Recall Queue (for responsiveness), Agent achieves a balanced architecture capable of handling both fire-and-forget operations and synchronous collaborative dialogues.

### 2.5 Task Decomposition

#### 2.5.1 ViewSpace And ViewSpace Tree

In NucleusMem, complex tasks are decomposed and executed through a structure called the **ViewSpace Tree**. The core insight behind this design is that **the boundary of a task and the boundary of its collaboration space should be the same thing**. Each node in the tree is simultaneously a task decomposition unit and an isolated execution environment.

A ViewSpace is defined as:

```tex
ViewSpace = Represented Agent (coordinator)
          + Worker Agents (executors)  
          + MemSpace (shared blackboard)
          + Input/Output Contract
```

This is analogous to a self-contained "room" where a group of agents collaborate on a bounded sub-problem. All communication within a ViewSpace happens through its dedicated MemSpace, following the blackboard pattern described in §1.2.2. No direct message passing between agents is required.

**The ViewSpace Tree**

A task decomposition is represented as a **ViewSpace Tree**: a rooted, ordered tree where each node is a ViewSpace. The tree is dynamically generated at runtime by the Represented Agent of the root node, and may be recursively deepened as composite nodes spawn their own children.

```
Root ViewSpace (Global)
├── Child A (Process)
│   ├── Child A1 (Atomic)  ← actual work happens here
│   └── Child A2 (Atomic)
└── Child B (Atomic)
```

![ViewTree](https://github.com/cheng-zhangpei/NucleusMem/blob/main/doc/img/ViewTree.png)

**The Formation of the ViewSpace in ViewSpace Tree**

- Global ViewSpace

​		 A global task feedback queue will be maintained within the task area in the memspace of Global ViewSpace. This queue records how tasks are decomposed—either provided statically by users or decomposed by the LLM according to specific paradigms. Each node in the queue maintains a Done channel to receive the task's completion status and communication methods for Process ViewSpace.

​		For task auditing purposes, Global ViewSpace must maintain comprehensive tool usage logs and Agent decisions. Therefore, it must be bound to all Agents (this can be disabled under non-mandatory auditing conditions to avoid write amplification).

- Process ViewSpace

​		The intermediate nodes of the ViewSpace Tree serve to isolate permission domains, decompose tasks downward, and report task progress upward.

- Atomic ViewSpace

​		The nodes of the entire ViewSpace Tree that execute actual tools. Based on task requirements, RepresentedAgent reads tools from the specified (or autonomously selected) memspace within the Tool region, encapsulates them into a tool Task, and hands it over to the task queue. After the task queue executes the Task, it feeds the execution results back to the Process ViewSpace.

**Known Limitations and Open Problems**

The current design acknowledges several unresolved challenges that are deferred beyond the demo phase:

**Decomposition reliability**: The quality of the ViewSpace Tree is entirely dependent on the LLM's output. There is no built-in mechanism to evaluate whether a decomposition is wel8l-formed in terms of granularity consistency or completeness. Malformed decompositions (e.g., circular dependencies, mismatched contracts) must be caught at parse time and returned to the LLM for correction.

**Dynamic re-decomposition**: If a child ViewSpace discovers mid-execution that the task was incorrectly scoped, the protocol for modifying the tree (in-place mutation vs. subtree replacement) is not yet defined.

**Cross-sibling communication**: The current model routes all inter-sibling communication through the parent's MemSpace. For deeply nested trees this introduces latency and complexity that has not been fully analyzed.

#### 2.5.2 Regulation In Decomposition

The entire task decomposition process can be summarized in the Flow below:

```tex
user task → LLM decomposition → raw JSON → Parser  → regulation check → ViewSpace Tree
                         ↑                              │
                         └──── retry ←──────────────────┘
```

**1. Schema**

```json
{
  "meta": { ... },
  "viewspaces": [ ... ],
  "dependencies": [ ... ]
}
```

**1.1 Meta Definition**

Meta is the meta data of the task

```json
{
  "meta": {
    "task_id": "code-quality-review-001",
    "description": "Analyze the code quality of Project X and provide recommendations for improvement.",
    "created_by": "user | llm",
    "max_retry": 3,
    "global_constraints": {
      "audit_mode": "full | summary"
    }
  }
}
```

**1.2 ViewSpace Definition**

```json
{
  "viewspaces": [
    // Global ViewSpace
    {
      "name": "root",
      "type": "global",
      "tags": ["code-review", "management"],
      "description": "Global Coordination: Receive results from all subtasks and generate the final report.",
      "represented_agent": {
        "AgentID":xxx
        "role": "Technical Director",
      },
    },
    // Process ViewSpace
    {
      "name": "analysis-group",
      "type": "process",
      "tags": ["code-review", "analysis", "coordination"],
      "description": "Coordinate all analytical subtasks, isolate analytical domain permissions, and consolidate analytical results.",
      "represented_agent": {
        "role": "Head of the Analysis Team"
      },
      "worker_agents": [
        {"AgentID": xxx, "role": "Analysis"},
      ],
      "memspace_selector": {
        "match_tags": ["static-analysis", "code-tools"],
        "prefer_tags": ["high-performance"]
      },
    },
    // Atomic ViewSpace
    {
      "name": "static-analysis",
      "type": "atomic",
      "tags": ["code-review", "lint", "metrics"],
      "description": "Run static analysis tools to collect code complexity metrics and identify violations of coding standards.",
      "represented_agent": {
        "AgentID":xxx
        "role": "Static Analysis Engineer"
      },
      "worker_agents": [
        {"AgentID": xxx, "role": "Analysis"},
      ],
      "tools": {
        "required": ["lint", "complexity_analyzer"],
      },
      "memspace_selector": {
        "match_tags": ["static-analysis", "code-tools"],
        "prefer_tags": ["high-performance"]
      },
    },
  ]
}
```

**1.3 Dependency Definition**

```json
{
  "dependencies": {
      // the structure of the ViewSpace Tree
    "tree": [
      { "parent": "root",           "children": ["analysis-group", "synthesis"] },
      { "parent": "analysis-group", "children": ["static-analysis", "security-scan"] }
    ],
      // the dataflow between the ViewSpace
    "dataflow": [
      {
        "from": "static-analysis",
        "to": "synthesis",
        "fields": ["metrics", "violations"],
        "description": "Static analysis metric data flows into the comprehensive report"
      },
      {
        "from": "security-scan",
        "to": "synthesis",
        "fields": ["vulnerabilities", "severity_summary"],
        "description": "Security scan results are incorporated into the comprehensive report."
      }
    ]
  }
}
```
#### 2.5.3 ViewSpace Tree Validation Rules

The ViewSpace Tree decomposition engine enforces a set of structural and semantic rules to ensure the generated task graph is valid, executable, and logically sound. These rules are categorized into Hard Rules (validation errors) and Soft Rules (warnings).

- Hard Rules

Hard rules must be satisfied for the TaskDefinition to be considered valid. Violations result in parsing errors that are returned to the LLM for correction.

| Rule ID | Name | Description | Failure Condition |
| :--- | :--- | :--- | :--- |
| **META** | Meta Information | The meta section must contain a valid task identifier. | `meta.task_id` is missing or empty. |
| **H1** | Unique Global Node | The tree must have exactly one root node of type `global`. | Zero or more than one `global` node exists. |
| **H2** | Atomic Leaf Nodes | Nodes of type `atomic` represent leaf tasks and cannot have children. | An `atomic` node appears as a parent in the `tree` dependencies. |
| **H3** | Unique Names | All ViewSpace names must be unique across the entire definition. | Duplicate names found in the `viewspaces` list. |
| **H3b** | Non-Leaf Children | Nodes of type `global` and `process` must coordinate sub-tasks. | A `global` or `process` node has no children in the `tree` dependencies. |
| **H4** | Tree Connectivity | All nodes must be reachable from the `global` root node. | One or more nodes are disconnected from the main tree. |
| **H5** | Acyclic Dataflow | Data dependencies must not form circular loops. | A cycle is detected in the `dataflow` edges. |
| **H8** | Reference Existence | All nodes referenced in dependencies must be defined. | A `parent`, `child`, `from`, or `to` field references an undefined name. |
| **TOOL-1** | Atomic Tools | Atomic nodes represent tool execution and must specify tools. | An `atomic` node has an empty or missing `tools` list. |
| **TOOL-2** | Coordination Tools | Coordination nodes (`global`, `process`) do not execute tools directly. | A `global` or `process` node contains a `tools` list. |

- Soft Rules

Soft rules are heuristic checks designed to optimize task structure for performance and maintainability. Violations generate warnings but do not block execution.

| Rule ID | Name | Threshold | Recommendation |
| :--- | :--- | :--- | :--- |
| **S2** | Tree Depth | Depth <= 4 | Deep trees increase coordination latency. Consider flattening the structure. |
| **S3** | Fan-out Limit | Children <= 10 | Nodes with too many children are hard to coordinate. Consider grouping into sub-processes. |

- Validation Flow

The validation process occurs in three phases:

1.  **JSON Decoding**: The raw output is parsed into the `TaskDefinition` struct. Syntax errors are caught here.
2.  **Hard Rule Checking**: All hard rules (META, H1-H8, TOOL) are evaluated. If any fail, validation stops and errors are returned.
3.  **Soft Rule Checking**: If hard rules pass, soft rules (S2, S3) are evaluated. Warnings are logged but do not invalidate the definition.

- Error Feedback Format

When validation fails, the errors are formatted into a human-readable string and fed back to the LLM for correction.

Example error feedback:

```text
Your ViewSpace Tree has the following errors. Please fix them and output the corrected JSON:

1. [H1] no global viewspace found, exactly 1 required
2. [H2] node 'code-scan': atomic viewspace cannot have children
3. [TOOL] node 'report': process viewspace should not have tools, only atomic nodes have tools
```


#### 2.5.3 Runtime of ViewSpaceTree

This section describes the execution model and runtime behavior of the ViewSpaceTree, including node lifecycle management, task scheduling, and tool execution strategies.

##### 2.5.3.1 Overview

The ViewSpaceTree runtime is responsible for transforming a logically decomposed task graph into actual execution. Its core responsibilities are:

1. **Node Lifecycle Management**: Allocate and initialize physical resources (MemSpace + Agent) for each logical ViewSpace node.
2. **Topological Scheduling**: Execute nodes respecting dataflow dependencies defined in `ChildDataflow`.
3. **Tool Execution**: Run atomic operations (tools) either sequentially or concurrently based on dependency graphs.
4. **Result Aggregation**: Collect outputs from child nodes and propagate them upward for final synthesis.

The runtime operates in two distinct phases:
- **Grow Phase** (`pkg/viewspace/grow.go`): Recursively decomposes tasks and spawns child ViewSpaces.
- **Execute Phase** (`pkg/viewspace/execute.go`): Traverses the tree bottom-up, executing nodes and aggregating results.

##### 2.5.3.2 Execution Model: Recursion + Topological Scheduling

The execution logic is implemented as a recursive descent with concurrent scheduling at each composite node.

```go
// Simplified execution flow
func (t *ViewSpaceTree) executeNode(node *ViewSpaceNode) (*NodeResult, error) {
    switch node.Type {
    case "atomic":
        return t.executeAtomic(node)      // Leaf: run tools
    case "process", "global":
        return t.executeComposite(node)   // Internal: schedule children
    }
}
```

- Composite Node Scheduling (`executeComposite`)

For `process` and `global` nodes, children are scheduled based on a **local DAG** derived from `ChildDataflow`:

1. **Dependency Map Construction**: Build `childName -> [dependencies]` from dataflow edges.
2. **Ready Queue Initialization**: Identify children with no unsatisfied dependencies.
3. **Concurrent Launch**: Start ready children in separate goroutines.
4. **Dynamic Ready Check**: As each child completes, re-evaluate which children have become ready.
5. **Result Aggregation**: Collect all child results and compute node-level output.

This approach enables **maximal concurrency within a node's scope** while respecting explicit data dependencies. It is intentionally localized: each composite node only schedules its direct children, not the entire subtree. This keeps scheduling logic simple and avoids global state complexity.

- Atomic Node Execution (`executeAtomic`)

Atomic nodes represent leaf tasks that invoke external tools. The current implementation supports two execution modes:

| Mode                     | Trigger                                           | Behavior                                                     |
| ------------------------ | ------------------------------------------------- | ------------------------------------------------------------ |
| **Sequential**           | `node.Tools` list defined, no ToolDAG in MemSpace | Tools executed one-by-one via Agent HTTP API                 |
| **Concurrent (ToolDAG)** | ToolDAG found in MemSpace                         | Submit `tool_dag` task to Agent; Agent performs topological scheduling |

The ToolDAG mode delegates concurrency to the Agent layer, which loads the DAG from MemSpace and executes tools with dependency-aware parallelism (see §2.4.1).

##### 2.5.3.3 Node Lifecycle: The `bringToLife`

The `bringToLife` function (`pkg/viewspace/grow.go`) embodies the principle that **a logical task node must be "born" into a physical execution environment before it can do work**.

```go
func (t *ViewSpaceTree) bringToLife(node *ViewSpaceNode) error {
    // 1. Launch MemSpace (persistent state + blackboard)
    // 2. Launch Agent (stateless compute + LLM interface)
    // 3. Bind Agent ↔ MemSpace via Manager
    // 4. Create direct HTTP clients for low-latency communication
    // 5. Mark node as Ready
}
```

- Design Rationale

| Step                 | Purpose                                                      | Why It Matters                                               |
| -------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| **Launch MemSpace**  | Provides persistent memory, tool registry, and communication blackboard | Enables state inheritance, auditability, and multi-agent collaboration |
| **Launch Agent**     | Provides LLM reasoning, task parsing, and tool invocation    | Separates compute from storage; agents remain stateless and replaceable |
| **Bind via Manager** | Establishes routing metadata in the cluster                  | Allows dynamic discovery without hard-coded addresses        |
| **Direct Clients**   | Node holds `*MemSpaceClient` and `*AgentClient`              | Avoids Manager proxy overhead for frequent operations        |

- Semantic Fields for Collaboration

To support complex collaboration patterns, `ViewSpaceNode` includes optional semantic fields:

```go
type ViewSpaceNode struct {
    // ... identity and topology fields ...
    
    // Collaboration context
    RepresentedAgentID uint64   `json:"represented_agent_id"`   // Primary coordinator
    WorkerAgentIDs     []uint64 `json:"worker_agent_ids,omitempty"`  // Contributing agents
    SharedMemSpaceIDs  []uint64 `json:"shared_memspace_ids,omitempty"` // Inherited/shared memory spaces
}
```

These fields are populated during `bringToLife` based on the decomposition output and enable:
- **Permission isolation**: Private MemSpaces restrict access to `RepresentedAgentID`
- **Resource sharing**: `SharedMemSpaceIDs` allow child nodes to read parent context
- **Audit scoping**: Global ViewSpace can trace actions across all `WorkerAgentIDs`

##### 2.5.3.4 Tool Execution: From Sequential to DAG-Concurrent

- Sequential Mode (Legacy)

When no ToolDAG is present, tools are executed sequentially via the Agent's `SubmitTask` API:

```
ViewSpace → Agent.SubmitTask(tool) → Agent invokes executor → Result returned
```

This mode is simple and sufficient for independent tools but does not exploit parallelism.

- Concurrent Mode (ToolDAG)

When a ToolDAG is injected into the node's MemSpace (typically during Grow), execution follows a delegated pattern:

```
ViewSpace → Agent.SubmitTask(type="tool_dag", memspace_id=X)
         → Agent loads DAG from MemSpace X
         → Agent.ToolDAGExecutor: topological schedule + concurrent HTTP calls
         → Agent aggregates results → Returns JSON string
         → ViewSpace parses and aggregates output
```

**Key properties**:
- **Dependency-aware**: Tools with no unsatisfied dependencies start concurrently.
- **Fault isolation**: A failed tool does not block unrelated parallel branches.
- **Audit-native**: Each tool execution is recorded in MemSpace's ToolRegion.

##### 2.5.3.5 Collaboration Protocol: Agent ↔ MemSpace Interaction

All inter-node communication and state management flows through MemSpace, following the "Memory as Communication" principle (§1.2.2).

- Data Flow During Execution

```
[Parent Process Node]
        │
        ▼
[Child Atomic Node]
        │
        ├──► MemSpace: Write tool output to MemoryRegion
        ├──► MemSpace: Write execution record to ToolRegion
        └──► MemSpace: Notify via CommRegion (if configured)
        │
        ▼
[Parent] reads child results from MemSpace or direct RPC
```

- Key Guarantees

| Guarantee          | Mechanism                                                    |
| ------------------ | ------------------------------------------------------------ |
| **Consistency**    | Percolator 2PC via TinyKV client ensures atomic writes across regions |
| **Isolation**      | MemSpace Key prefixing (`/viewspace/{id}/...`) prevents cross-node leakage |
| **Traceability**   | All writes are timestamped and versioned; history is queryable |
| **Recoverability** | Node state is persisted; restart resumes from last committed state |

##### 2.5.3.6 Current Limitations

The current runtime implementation has several known constraints:

1. **Single-Machine Simulation**: Concurrency is achieved via goroutines on one host; true distributed scheduling (across nodes) is deferred to the Operator phase.
2. **Static Resource Allocation**: `bringToLife` launches new MemSpace/Agent processes for each node; reuse of idle resources is not yet implemented.
3. **No Dynamic Re-decomposition**: If a child node discovers mid-execution that its task scope is incorrect, the tree cannot be mutated in-place.
4. **Simplified Error Propagation**: Failure in one child marks the parent as `partial`; fine-grained retry or fallback strategies are not yet supported.

These limitations are acknowledged and are targets for future iteration as the system evolves toward production readiness.

##### 2.5.3.7 Future Directions

Planned enhancements to the ViewSpaceTree runtime include:

- **Resource-Aware Scheduling**: Query global resource pools during `bringToLife` to reuse existing MemSpaces/Agents when possible.
- **Dynamic Re-decomposition Protocol**: Allow subtree replacement or in-place mutation when execution reveals decomposition errors.
- **Cross-Node Distributed Execution**: Integrate with Kubernetes Operator to schedule ViewSpace nodes across a cluster.
- **Adaptive Concurrency**: Adjust parallelism based on real-time load metrics and tool execution latency.

These improvements aim to transition the runtime from a functional prototype to a scalable, production-grade orchestration layer for large-scale multi-agent systems.





> **todo**: after the demo finished.
>
> *Known open problems to address in this section:*
>
> - *Read/write permission model across nested ViewSpace boundaries (a child ViewSpace should not be able to write to its parent's or sibling's MemSpace)*
> - *How permission grants are propagated when a parent passes a reference to a child*
> - *Security audit trail: since all state lives in MemSpace, audit is structurally possible but the access control model needs to be formally defined first*



#### 2.5.4 Task Region

The task region exists in two distinct forms for Atomic ViewSpaces and Process ViewSpaces. The former stores the dependency graph of tools within that ViewSpace, while the latter stores the dependency graph between ViewSpaces.


