## Install

go > 1.23.1

```shell
    git submodule update --init --recursive
    make build
```

## Makefile에서 테스트 계정수 변경

```make
    # 테스트 계정수 세팅
    NUM_ACCOUNTS := 50
```

## Running Node

```shell
    # run sigle node
    make start-single

    #run dual node
    make start-dual

    #run triple node
    make start-triple

    #run quad node
    make start-quad
```

## Tps 측정

```shell
    # 테스트 계정 수만큼의 singed tx(serialized) 세팅, 첫 블록 생성전까지 잠시 기달
    make reload

    # 트잭 전송 및 측정
    make shot tps=500 runtime=600

```

# metric

```shell
    make metric
```

# Result

```json
<tx_results.json>
{"total_sent":1000,"total_succeed":1000,"tps":9.602181615663078,"total_latency":104143}
{"send_timestamp":1736698779859,"commit_timestamp":1736698780000,"latency":141,"block_height":602,"tx_hash":"E890816812B6A0B1D8588873DCFC8B6767201066E1C18F68D4048915A54203B9"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"EA224B9A569C2051DB7AE13F6B18D343AAF01806BC81F76CA44D4C4E38E3D5C7"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"E4240BFF3DCCD57541C00EEB297EB8371307332313FA4EF3D6C35D1445DADFF6"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"B559E510BC202A9DEAC92C4A2706CE8AA37F767ED9188D6A703B8D9CB1E2E8BD"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"7CA760A129579C723F1E68F44155F8E1649E08D1341ACB7277F7BFF763C2C891"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"244513D4B59A4FDE45E5EDD93FDCA94A294BAEFF7384DE53A6DC3D269AA0DB38"}
{"send_timestamp":1736698779860,"commit_timestamp":1736698780000,"latency":140,"block_height":602,"tx_hash":"BCE3F993B4740065C2A9964D83CC1CCA517B29DB23198339F07301F50D05BAE4"}
{"send_timestamp":1736698779859,"commit_timestamp":1736698780000,"latency":141,"block_height":602,"tx_hash":"A5F5BAD4F3938F9B7E74A8F8565646C50F7BAA7EEAC24207A969E9F1CC0247E1"}
.
.
.

```
