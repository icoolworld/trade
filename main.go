package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// 定义三个交易对的符号
const (
	FIL_ETH = "FIL-ETH"
	ETH_BSV = "ETH-BSV"
	FIL_BSV = "FIL-BSV"
)

// 定义三个交易对的价格信息结构体
type PriceInfo struct {
	Symbol string  `json:"symbol"` // 交易对符号
	Bid    float64 `json:"bid"`    // 买一价
	Ask    float64 `json:"ask"`    // 卖一价
}

// 定义三角套利策略结构体
type TriArbStrategy struct {
	FilAmount float64 // 初始的FIL币量
	EthAmount float64 // 初始的ETH币量
	BsvAmount float64 // 初始的BSV币量
	CostRate  float64 // 交易手续费率
	SlipRate  float64 // 滑点率
}

// 定义三角套利策略的方法，根据三个交易对的价格信息，判断是否存在套利机会，如果有，执行相应的交易操作，并更新币量
func (s *TriArbStrategy) Execute(prices map[string]*PriceInfo) {
	// 获取三个交易对的价格信息
	filEthPrice := prices[FIL_ETH]
	ethBsvPrice := prices[ETH_BSV]
	filBsvPrice := prices[FIL_BSV]

	if filEthPrice == nil || ethBsvPrice == nil || filBsvPrice == nil {
		return // 如果有任何一个价格信息缺失，直接返回
	}

	// 计算正向套利条件：FIL/ETH的卖一价 * ETH/BSV的卖一价 < FIL/BSV的买一价 * (1 - 手续费率) * (1 - 滑点率)
	forwardArbCond := filEthPrice.Ask*ethBsvPrice.Ask < filBsvPrice.Bid*(1-s.CostRate)*(1-s.SlipRate)

	// 计算反向套利条件：FIL/ETH的买一价 * ETH/BSV的买一价 > FIL/BSV的卖一价 * (1 + 手续费率) * (1 + 滑点率)
	reverseArbCond := filEthPrice.Bid*ethBsvPrice.Bid > filBsvPrice.Ask*(1+s.CostRate)*(1+s.SlipRate)

	if forwardArbCond {
		// 如果存在正向套利机会，执行以下操作：
		// 1. 用一部分FIL币（比如100个）去买ETH，假设FIL/ETH的卖一价是0.1，那么可以得到10个ETH（扣除手续费和滑点）
		filToSell := 100.0                      // 要卖出的FIL币量
		filCost := filToSell / (1 - s.CostRate) // 实际要花费的FIL币量（加上手续费）
		ethToBuy := filToSell * filEthPrice.Ask // 要买入的ETH币量
		ethGet := ethToBuy * (1 - s.SlipRate)   // 实际得到的ETH币量（扣除滑点）
		s.FilAmount -= filCost                  // 更新FIL币量
		s.EthAmount += ethGet                   // 更新ETH币量
		fmt.Printf("用%.2f个FIL买入%.2f个ETH\n", filCost, ethGet)

		// 2. 然后用这10个ETH去买BSV，假设ETH/BSV的卖一价是0.5，那么可以得到5个BSV（扣除手续费和滑点）
		ethToSell := ethGet                     // 要卖出的ETH币量
		ethCost := ethToSell / (1 - s.CostRate) // 实际要花费的ETH币量（加上手续费）
		bsvToBuy := ethToSell * ethBsvPrice.Ask // 要买入的BSV币量
		bsvGet := bsvToBuy * (1 - s.SlipRate)   // 实际得到的BSV币量（扣除滑点）
		s.EthAmount -= ethCost                  // 更新ETH币量
		s.BsvAmount += bsvGet                   // 更新BSV币量
		fmt.Printf("用%.2f个ETH买入%.2f个BSV\n", ethCost, bsvGet)

		// 3. 最后用这5个BSV去买回FIL币，假设FIL/BSV的买一价是0.04，那么可以得到125个FIL币（扣除手续费和滑点）
		bsvToSell := bsvGet                     // 要卖出的BSV币量
		bsvCost := bsvToSell / (1 - s.CostRate) // 实际要花费的BSV币量（加上手续费）
		filToBuy := bsvToSell * filBsvPrice.Bid // 要买入的FIL币量
		filGet := filToBuy * (1 - s.SlipRate)   // 实际得到的FIL币量（扣除滑点）
		s.BsvAmount -= bsvCost                  // 更新BSV币量
		s.FilAmount += filGet                   // 更新FIL币量
		fmt.Printf("用%.2f个BSV买入%.2f个FIL\n", bsvCost, filGet)

		fmt.Printf("完成一次正向套利，FIL币量从%.2f增加到了%.2f\n", filToSell, filGet)
	}

	if reverseArbCond {
		// 如果存在反向套利机会，执行以下操作：
		// 1. 用一部分FIL币（比如100个）去卖BSV，假设FIL/BSV的卖一价是0.04，那么可以得到4个BSV（扣除手续费和滑点）
		filToSell := 100.0                      // 要卖出的FIL币量
		filCost := filToSell / (1 - s.CostRate) // 实际要花费的FIL币量（加上手续费）
		bsvToBuy := filToSell * filBsvPrice.Ask // 要买入的BSV币量
		bsvGet := bsvToBuy * (1 - s.SlipRate)   // 实际得到的BSV币量（扣除滑点）
		s.FilAmount -= filCost                  // 更新FIL币量
		s.BsvAmount += bsvGet                   // 更新BSV币量
		fmt.Printf("用%.2f个FIL卖出%.2f个BSV\n", filCost, bsvGet)

		// 3. 最后用这2个ETH去买回FIL币，假设FIL/ETH的买一价是0.1，那么可以得到20个FIL币（扣除手续费和滑点）
		ethToSell := ethGet                     // 要卖出的ETH币量
		ethCost := ethToSell / (1 - s.CostRate) // 实际要花费的ETH币量（加上手续费）
		filToBuy := ethToSell * filEthPrice.Bid // 要买入的FIL币量
		filGet := filToBuy * (1 - s.SlipRate)   // 实际得到的FIL币量（扣除滑点）
		s.EthAmount -= ethCost                  // 更新ETH币量
		s.FilAmount += filGet                   // 更新FIL币量
		fmt.Printf("用%.2f个ETH买入%.2f个FIL\n", ethCost, filGet)

		fmt.Printf("完成一次反向套利，FIL币量从%.2f增加到了%.2f\n", filToSell, filGet)
	}
}

// 定义一个函数，用于连接交易所的websocket API，并接收三个交易对的价格信息
func connectAndReceivePrices(url string, prices chan map[string]*PriceInfo) {
	// 创建一个websocket客户端
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(request.URL.String(), request.Header)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		// 接收消息
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		if messageType == websocket.TextMessage {
			// 解析消息为价格信息结构体
			var price PriceInfo
			err = json.Unmarshal(message, &price)
			if err != nil {
				log.Println(err)
				continue
			}

			if price.Symbol == FIL_ETH || price.Symbol == ETH_BSV || price.Symbol == FIL_BSV {
				// 如果是我们关注的三个交易对之一，就把价格信息发送到通道中
				prices <- map[string]*PriceInfo{price.Symbol: &price}
			}
		}
	}
}

func main() {
	// 定义一个通道，用于接收三个交易对的价格信息
	prices := make(chan map[string]*PriceInfo)

	// 定义一个交易所的websocket API地址（这里只是示例，实际地址可能不同）
	url := "wss://example.com/ws"

	// 启动一个协程，连接交易所的websocket API，并接收三个交易对的价格信息
	go connectAndReceivePrices(url, prices)

	// 创建一个三角套利策略实例，假设初始有700个FIL币，没有其他币，交易手续费率是0.1%，滑点率是0.01%
	strategy := &TriArbStrategy{
		FilAmount: 700.0,
		EthAmount: 0.0,
		BsvAmount: 0.0,
		CostRate:  0.001,
		SlipRate:  0.0001,
	}

	// 定义一个map，用于存储三个交易对的最新价格信息
	priceMap := make(map[string]*PriceInfo)

	for {
		// 从通道中接收一个价格信息
		price := <-prices

		// 更新map中对应的价格信息
		for symbol, info := range price {
			priceMap[symbol] = info
		}

		// 执行三角套利策略
		strategy.Execute(priceMap)

		// 每隔一段时间（比如10秒），打印一下当前的币量
		ticker := time.NewTicker(10 * time.Second)
		select {
		case <-ticker.C:
			fmt.Printf(
				"当前的币量：FIL=%.2f, ETH=%.2f, BSV=%.2f\n",
				strategy.FilAmount,
				strategy.EthAmount,
				strategy.BsvAmount,
			)
			ticker.Stop()
		default:
			continue
		}
	}
}
