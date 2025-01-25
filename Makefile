# OS 감지  
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)  
    SED_CMD = sed -i ''
else  
    SED_CMD = sed -i
endif  

# 테스트 계정수
NUM_ACCOUNTS := 1000

# 기본 설정
BIN_PATH := ./bin
BASE_PATH := ./node
CHAINID := cronos_777-1
MONIKER := localtestnet
KEYRING := test
KEYALGO := eth_secp256k1
LOGLEVEL := info
TRACE := --trace

# 기본 포트 
BASE_P2P_PORT := 26656
BASE_RPC_PORT := 26657# Tendermint RPC 
BASE_API_PORT := 1317# Cosmos REST Api
BASE_GRPC_PORT := 9090
BASE_JSONRPC_PORT := 8545
BASE_WS_PORT := 9545
PORT_ADDITION := 0

# 노드 수
SIGLE_NODE := 1
DUAL_NODE := 2
TRIPLE_NODE := 3
QUAD_NODE := 4

# .PHONY 타겟 설정  
.PHONY: _clean_nodes start-single start-dual start-triple start-quad stop status build

build:
	@mkdir -p bin 
	@if [ ! -f bin/cronosd ]; then \
		cd cronos && $(MAKE) build && mv build/cronosd ../bin; \
	fi
	@go build -o bin/gen-key ./cmd/gen-key  
	@go build -o bin/gen-tx ./cmd/gen-tx  
	@go build -o bin/send-tx ./cmd/send-tx
	@go build -o bin/add-genesis-account ./cmd/add-genesis-account

# 초기화
define init_nodes  
	@echo "Initializing nodes..."
	@for i in $$(seq 1 $(1)); do \
		echo "Initializing node$$i..."; \
		mkdir -p $(BASE_PATH)/node$$i; \
		mkdir -p logs; \
		$(BIN_PATH)/cronosd init $(MONIKER) --chain-id $(CHAINID) --home $(BASE_PATH)/node$$i; \
	done  
endef  

# 제네시스 설정 함수  
define setup_genesis
	@echo "Setting up genesis..."  	
	
	# 토큰 단위명 수정
	cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="stake"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
    cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="stake"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
    cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="stake"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
    cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["mint"]["params"]["mint_denom"]="stake"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
    cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["evm"]["params"]["evm_denom"]="stake"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
	# base fee 없애기
    cat $(BASE_PATH)/node1/config/genesis.json | jq '.app_state["feemarket"]["params"]["no_base_fee"]=true' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json

	cat $(BASE_PATH)/node1/config/genesis.json | jq '.consensus["params"]["block"]["time_iota_ms"]="1000"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json
	cat $(BASE_PATH)/node1/config/genesis.json | jq '.consensus["params"]["block"]["max_gas"]="100000000"' > $(BASE_PATH)/node1/config/tmp_genesis.json && mv $(BASE_PATH)/node1/config/tmp_genesis.json $(BASE_PATH)/node1/config/genesis.json

	@for i in $$(seq 1 $(1)); do \
		$(BIN_PATH)/cronosd keys add node$${i}key --keyring-backend $(KEYRING) --algo $(KEYALGO) --home $(BASE_PATH)/node$${i}; \
		if [ "$${i}" != "1" ]; then \
			cp $(BASE_PATH)/node$${i}/keyring-$(KEYRING)/* $(BASE_PATH)/node1/keyring-$(KEYRING); \
		fi; \
		$(BIN_PATH)/cronosd genesis add-genesis-account node$${i}key --keyring-backend $(KEYRING) 100000000000000000000000000stake --home $(BASE_PATH)/node1; \
		if [ "$${i}" != "1" ]; then \
			cp $(BASE_PATH)/node1/config/genesis.json $(BASE_PATH)/node$${i}/config/genesis.json; \
		fi; \
		$(BIN_PATH)/cronosd genesis gentx node$${i}key 1000000000000000000000stake --keyring-backend $(KEYRING) --chain-id $(CHAINID) --home $(BASE_PATH)/node$${i}; \
		if [ "$${i}" != "1" ]; then \
			cp $(BASE_PATH)/node$${i}/config/gentx/* $(BASE_PATH)/node1/config/gentx/; \
		fi; \
	done

	$(BIN_PATH)/gen-key -a $(NUM_ACCOUNTS)
	$(BIN_PATH)/add-genesis-account -a $(NUM_ACCOUNTS)
	$(BIN_PATH)/cronosd genesis collect-gentxs --home $(BASE_PATH)/node1
	$(BIN_PATH)/cronosd genesis validate --home $(BASE_PATH)/node1
endef  

# 단일 노드 실행  
start-single: 
	@$(MAKE) _clean_nodes
	@$(call init_nodes,$(SIGLE_NODE))
	@$(call setup_genesis,$(SIGLE_NODE))
	@echo "Starting single node..."
	@$(MAKE) _start_node NODE_NUM=1
	@echo '{"node_num" : "1"}' > cache.json
	@echo "Single node setup complete"

# 2개 노드 실행  
start-dual: 
	@$(MAKE) _clean_nodes
	@$(call init_nodes,$(DUAL_NODE))
	@$(call setup_genesis,$(DUAL_NODE))
	@echo "Starting dual nodes..."
	@$(MAKE) _start_node NODE_NUM=1
	@sleep 2
	@$(MAKE) _start_node NODE_NUM=2
	@echo '{"node_num" : "2"}' > cache.json
	@echo "Dual nodes setup complete"

# 3개 노드 실행  
start-triple:
	@$(MAKE) _clean_nodes
	@$(call init_nodes,$(TRIPLE_NODE))
	@$(call setup_genesis,$(TRIPLE_NODE))
	@echo "Starting triple nodes..."
	@$(MAKE) _start_node NODE_NUM=1
	@sleep 2
	@$(MAKE) _start_node NODE_NUM=2
	@$(MAKE) _start_node NODE_NUM=3
	@echo '{"node_num" : "3"}' > cache.json
	@echo "Triple nodes setup complete"

# 4개 노드 실행  
start-quad:
	@$(MAKE) _clean_nodes
	@$(call init_nodes,$(QUAD_NODE))
	@$(call setup_genesis,$(QUAD_NODE))
	@echo "Starting quad nodes..."
	@$(MAKE) _start_node NODE_NUM=1
	@sleep 2
	@$(MAKE) _start_node NODE_NUM=2
	@$(MAKE) _start_node NODE_NUM=3
	@$(MAKE) _start_node NODE_NUM=4
	@echo '{"node_num" : "4"}' > cache.json
	@echo "Quad nodes setup complete"

reload:
	$(BIN_PATH)/gen-tx --a $(NUM_ACCOUNTS)


tps ?= 500  
runtime ?= 600  
shot:  
	@NODE_NUM=$$(cat cache.json | jq -r '.node_num') && \
	go run ./cmd/send-tx/main.go $(tps) $(runtime)  

metric:
	go run ./cmd/update-height/main.go && \
	go run ./cmd/metrics/main.go

# 내부 노드 시작 함수  
_start_node:  
	@echo "Starting node $(NODE_NUM)..."  

	@if [ "$(NODE_NUM)" != "1" ]; then \
		cp $(BASE_PATH)/node1/config/genesis.json $(BASE_PATH)/node$(NODE_NUM)/config/genesis.json; \
	fi  

	# config.toml
	$(SED_CMD) 's/addr_book_strict = true/addr_book_strict = false/g' $(BASE_PATH)/node$(NODE_NUM)/config/config.toml  
	$(SED_CMD) 's/pex = true/pex = false/g' $(BASE_PATH)/node$(NODE_NUM)/config/config.toml  
	$(SED_CMD) 's/allow_duplicate_ip = false/allow_duplicate_ip = true/g' $(BASE_PATH)/node$(NODE_NUM)/config/config.toml  
	@if [ "$(NODE_NUM)" != "1" ]; then \
		$(SED_CMD) 's/pprof_laddr = "localhost:6060"/pprof_laddr = "127.0.0.1:606$(NODE_NUM)"/g' $(BASE_PATH)/node$(NODE_NUM)/config/config.toml; \
		$(SED_CMD) 's|rpc_servers = ""|rpc_servers = "http:\/\/127.0.0.1:$(BASE_JSONRPC_PORT)"|g' $(BASE_PATH)/node$(NODE_NUM)/config/config.toml; \
	fi  

	# app.toml
	@$(SED_CMD) 's|address = "tcp://localhost:1317"|address = "tcp://0.0.0.0:'$$(expr $(BASE_API_PORT) + \( $(NODE_NUM) - 1 \) \* 100)'"|g' $(BASE_PATH)/node$(NODE_NUM)/config/app.toml
	@$(SED_CMD) 's/swagger = false/swagger = true/g' $(BASE_PATH)/node$(NODE_NUM)/config/app.toml
	@$(SED_CMD) 's/max-txs = -1/max-txs = 100000/g' $(BASE_PATH)/node$(NODE_NUM)/config/app.toml

	@P2P_PEERS=""; \
	if [ "$(NODE_NUM)" != "1" ]; then \
		P2P_PEERS="--p2p.persistent_peers=$$($(BIN_PATH)/cronosd tendermint show-node-id --home $(BASE_PATH)/node1)@127.0.0.1:$(BASE_P2P_PORT)"; \
	fi; \
	$(BIN_PATH)/cronosd start \
		--home=$(BASE_PATH)/node$(NODE_NUM) \
		--moniker=node$(NODE_NUM) \
		--api.enable \
		--grpc-web.enable \
		--grpc.enable \
		--p2p.laddr=tcp://0.0.0.0:$$(expr $(BASE_P2P_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		--p2p.external-address=tcp://127.0.0.1:$$(expr $(BASE_P2P_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		$$P2P_PEERS \
		--rpc.laddr=tcp://0.0.0.0:$$(expr $(BASE_RPC_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		--json-rpc.address=127.0.0.1:$$(expr $(BASE_JSONRPC_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		--json-rpc.ws-address=127.0.0.1:$$(expr $(BASE_WS_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		--grpc.address=127.0.0.1:$$(expr $(BASE_GRPC_PORT) + \( $(NODE_NUM) - 1 \) \* 100) \
		--db_dir=data \
		--pruning=nothing \
		--evm.tracer=json $(TRACE) \
		--log_level $(LOGLEVEL) \
		--minimum-gas-prices=0stake \
		--chain-id=$(CHAINID) \
		--keyring-backend=$(KEYRING) \
		--json-rpc.api eth,txpool,personal,net,debug,web3,miner 2>&1 | \
	./script/log_capture.sh $(NODE_NUM) &  
	@echo "Node $(NODE_NUM) started" 

_clean_nodes:
	@echo "Cleaning up..."
	-killall -INT cronosd 2>/dev/null || true
	rm -rf $(BASE_PATH)/node1/*
	rm -rf $(BASE_PATH)/node2/*
	rm -rf $(BASE_PATH)/node3/*
	rm -rf $(BASE_PATH)/node4/*
	rm -rf $(BASE_PATH)/txns/*
	rm -rf logs/*

# 노드 정지  
stop:
	@echo "Stopping nodes..."
	-killall -INT cronosd 2>/dev/null || true

status: 
	@echo "Checking nodes status..."  
	@for i in $$(seq 1 $(shell ls -d $(BASE_PATH)/node* 2>/dev/null | wc -l)); do \
		if [ -f "$(BASE_PATH)/node$$i/node$${i}.log" ]; then \
			echo "Node $$i"; \
			echo "  -catching_up: $$(curl -s http://localhost:$$(expr $(BASE_RPC_PORT) + \( $$i - 1 \) \* 100)/status | jq -r '.result.sync_info.catching_up')"; \
			echo "  -latest_block_height: $$(curl -s http://localhost:$$(expr $(BASE_RPC_PORT) + \( $$i - 1 \) \* 100)/status | jq -r '.result.sync_info.latest_block_height')"; \
			echo "  -connected_peers: $$(curl -s http://localhost:$$(expr $(BASE_RPC_PORT) + \( $$i - 1 \) \* 100)/net_info | jq '.result.n_peers')"; \
			echo "  -base_fee: $$($(BIN_PATH)/cronosd q feemarket base-fee --home $(BASE_PATH)/node$$i --output json | jq -r '.base_fee')"; \
			echo ""; \
		fi; \
	done

node-info: 
	@echo "Checking nodes info"  
	@for i in $$(seq 1 $(shell ls -d $(BASE_PATH)/node* 2>/dev/null | wc -l)); do \
		if [ -f "$(BASE_PATH)/node$$i/node$${i}.log" ]; then \
			echo "Node $$i"; \
			echo "  -key_address: $$($(BIN_PATH)/cronosd keys show node$${i}key --keyring-backend $(KEYRING) --home  $(BASE_PATH)/node$$i -a)"; \
			echo "  -ID: $$($(BIN_PATH)/cronosd tendermint show-node-id --home $(BASE_PATH)/node$$i)"; \
			echo "  -listen_addr: $$(curl -s http://localhost:$$(expr $(BASE_RPC_PORT) + \( $$i - 1 \) \* 100)/status | jq -r '.result.node_info.listen_addr')"; \
			echo "  -rpc_addr: $$(curl -s http://localhost:$$(expr $(BASE_RPC_PORT) + \( $$i - 1 \) \* 100)/status | jq -r '.result.node_info.other.rpc_address')"; \
			echo ""; \
		fi; \
	done

peers:
	@echo "Checking peers"  
	@echo "$$(curl -s http://localhost:26657/dump_consensus_state | jq -r '.result.peers')"

proposer:
	@echo "Checking proposer"  
	@echo "$$(curl -s http://localhost:26657/consensus_state | jq -r '.result.round_state.proposer')"

key-info:
	@for i in $$(seq 1 $(shell ls -d $(BASE_PATH)/node* 2>/dev/null | wc -l)); do \
		if [ -f "$(BASE_PATH)/node$$i/node$${i}.log" ]; then \
			echo "  -key_address: $$($(BIN_PATH)/cronosd keys export node$${i}key --keyring-backend test --unsafe --home $(BASE_PATH)/node$$i --unarmored-hex)"; \
		fi; \
	done

# feemarket
# $(BIN_PATH)/cronosd q feemarket base-fee
# $(BIN_PATH)/cronosd q bank balances