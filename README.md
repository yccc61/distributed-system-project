# **Distributed Systems Projects**

This repository contains projects from the **Distributed Systems** course at **Carnegie Mellon University**, implemented in **Go**. The projects explore key distributed computing concepts, including **fault tolerance, consensus algorithms, reliable communication protocols, and distributed storage systems**.

## **Project Structure**

### **1. Key-Value Database Server**  
A **concurrent key-value store** supporting `Put`, `Get`, `Delete`, and `Update` operations. The server handles multiple clients using **goroutines and channels**, ensuring **efficient request handling and thread safety**. It also tracks active and dropped clients and manages slow-reading clients with **buffered message queues**.

### **2. Distributed Bitcoin Miner**  
A distributed mining system that uses the **Live Sequence Protocol (LSP)** for reliable client-server communication over **UDP**. It distributes mining tasks among multiple clients, leveraging **fault tolerance and dynamic load balancing** for efficiency. Miners compute hash values in parallel to find the optimal nonce, while the system dynamically reassigns tasks to handle client failures and maintain performance.

### **3. Raft Consensus Algorithm**  
An implementation of the **Raft consensus algorithm** for achieving **fault-tolerant log replication** in distributed systems. It ensures **log consistency** across replicas through **leader election, log replication, and heartbeat mechanisms** using **RPC communication**. The system adapts to **node failures**, maintaining reliable and consistent operations.

### **4. CMUD - Distributed Multiplayer Online Game**  
A **multiplayer online game** that runs on a **geographically distributed key-value store**, ensuring **low-latency and high-availability** interactions. I implemented the **backend storage system**, which supports **eventual consistency** using an **actor-based model**, where independent actors process queries and sync updates across multiple servers. I developed the **query actor system**, enabling efficient `Get`, `Put`, and `List` operations while enforcing a **last-writer-wins** rule to resolve conflicts. Additionally, I implemented **local and remote syncing**, ensuring data consistency across distributed nodes, and integrated **RPC-based communication** between storage clients and backend servers to enable seamless game interactions.
