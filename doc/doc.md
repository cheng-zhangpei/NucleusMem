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

> really like the arch of tikv right? hhhh
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

#### 2.3.2 Dynamic Task Decomposition Model

### 2.4 Agent

At its core, Agent operates on a task queue-driven execution model. All external requests—such as user conversations, collaboration notifications, and more—are encapsulated as tasks and submitted to an internal queue. These tasks are then processed sequentially by background coroutines.

Each task carries a type identifier (e.g., temp_chat, comm, chat), enabling Agent to invoke distinct processing logic based on the type. This design makes adding new task types (such as tool invocations or scheduled tasks) remarkably straightforward—simply register a new handler.

Although tasks are processed in submission order, strict end-to-end sequencing cannot be guaranteed due to network and external service latency. However, this approximate ordering suffices for most scenarios (e.g., conversations, memory writes).
Agent itself does not store business state. All persistent data is managed through MemSpace, while Agent maintains only temporary conversation context and task scheduling logic.

## 3. Future



