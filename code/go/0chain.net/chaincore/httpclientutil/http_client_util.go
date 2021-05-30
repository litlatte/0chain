package httpclientutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/node"
	"0chain.net/chaincore/state"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	"0chain.net/core/util"
	"go.uber.org/zap"
)

/*
  ToDo: This is adapted from blobber code. Need to find a way to reuse this
*/

const maxRetries = 5

//SleepBetweenRetries suggested time to sleep between retries
const SleepBetweenRetries = 500

//TxnConfirmationTime time to wait before checking the status
const TxnConfirmationTime = 15

const clientBalanceURL = "v1/client/get/balance?client_id="
const txnSubmitURL = "v1/transaction/put"
const txnVerifyURL = "v1/transaction/get/confirmation?hash="
const specificMagicBlockURL = "v1/block/magic/get?magic_block_number="
const scRestAPIURL = "v1/screst/"
const magicBlockURL = "v1/block/get/latest_finalized_magic_block"
const finalizeBlockURL = "v1/block/get/latest_finalized"

//RegisterClient path to RegisterClient
const RegisterClient = "/v1/client/put"

var httpClient *http.Client

func init() {
	var transport *http.Transport
	transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   1 * time.Second,
			KeepAlive: 1 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     1 * time.Second,
		MaxIdleConnsPerHost: 5,
	}
	httpClient = &http.Client{Transport: transport}
}

//Signer for the transaction hash
type Signer func(h string) (string, error)

//ComputeHashAndSign compute Hash and sign the transaction
func (t *Transaction) ComputeHashAndSign(handler Signer) error {
	hashdata := fmt.Sprintf("%v:%v:%v:%v:%v", t.CreationDate, t.ClientID,
		t.ToClientID, t.Value, encryption.Hash(t.TransactionData))
	t.Hash = encryption.Hash(hashdata)
	var err error
	t.Signature, err = handler(t.Hash)
	if err != nil {
		return err
	}
	return nil
}

/////////////// Plain Transaction ///////////

//NewHTTPRequest to use in sending http requests
func NewHTTPRequest(method string, url string, data []byte, ID string, pkey string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	if ID != "" {
		req.Header.Set("X-App-Client-ID", ID)
	}
	if pkey != "" {
		req.Header.Set("X-App-Client-Key", pkey)
	}
	return req, err
}

//SendMultiPostRequest send same request to multiple URLs
func SendMultiPostRequest(urls []string, data []byte, ID string, pkey string) {
	wg := sync.WaitGroup{}
	wg.Add(len(urls))

	for _, u := range urls {
		go SendPostRequest(u, data, ID, pkey, &wg)
	}
	wg.Wait()
}

//SendPostRequest function to send post requests
func SendPostRequest(url string, data []byte, ID string, pkey string, wg *sync.WaitGroup) ([]byte, error) {
	//ToDo: Add more error handling
	if wg != nil {
		defer wg.Done()
	}
	req, err := NewHTTPRequest(http.MethodPost, url, data, ID, pkey)
	if err != nil {
		logging.N2n.Info("SendPostRequest failure", zap.String("url", url))
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if resp == nil || err != nil {
		logging.N2n.Error("Failed after multiple retries", zap.Int("retried", maxRetries))
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

//SendTransaction send a transaction
func SendTransaction(txn *Transaction, urls []string, ID string, pkey string) {
	for _, u := range urls {
		txnURL := fmt.Sprintf("%v/%v", u, txnSubmitURL)
		go sendTransactionToURL(txnURL, txn, ID, pkey, nil)
	}
}

//GetTransactionStatus check the status of the transaction.
func GetTransactionStatus(txnHash string, urls []string, sf int) (*Transaction, error) {
	//ToDo: Add more error handling
	numSuccess := 0
	numErrs := 0
	var errString string
	var retTxn *Transaction

	// currently transaction information an be obtained only from sharders
	for _, sharder := range urls {
		urlString := fmt.Sprintf("%v/%v%v", sharder, txnVerifyURL, txnHash)
		response, err := httpClient.Get(urlString)
		if err != nil {
			logging.N2n.Error("get transaction status -- failed", zap.Any("error", err))
			numErrs++
		} else {
			contents, err := ioutil.ReadAll(response.Body)
			if response.StatusCode != 200 {
				// logging.Logger.Error("transaction confirmation response code",
				// 	zap.Any("code", response.StatusCode))
				response.Body.Close()
				continue
			}
			if err != nil {
				logging.Logger.Error("Error reading response from transaction confirmation", zap.Any("error", err))
				response.Body.Close()
				continue
			}
			var objmap map[string]*json.RawMessage
			err = json.Unmarshal(contents, &objmap)
			if err != nil {
				logging.Logger.Error("Error unmarshalling response", zap.Any("error", err))
				errString = errString + urlString + ":" + err.Error()
				response.Body.Close()
				continue
			}
			if *objmap["txn"] == nil {
				e := "No transaction information. Only block summary."
				logging.Logger.Error(e)
				errString = errString + urlString + ":" + e
			}
			txn := &Transaction{}
			err = json.Unmarshal(*objmap["txn"], &txn)
			if err != nil {
				logging.Logger.Error("Error unmarshalling to get transaction response", zap.Any("error", err))
				errString = errString + urlString + ":" + err.Error()
			}
			if len(txn.Signature) > 0 {
				retTxn = txn
			}
			response.Body.Close()
			numSuccess++
		}
	}

	sr := int(math.Ceil((float64(numSuccess) * 100) / float64(numSuccess+numErrs)))
	// We've at least one success and success rate sr is at least same as success factor sf
	if numSuccess > 0 && sr >= sf {
		if retTxn != nil {
			return retTxn, nil
		}
		return nil, common.NewError("err_finding_txn_status", errString)
	}
	return nil, common.NewError("transaction_not_found", "Transaction was not found on any of the urls provided")
}

func sendTransactionToURL(url string, txn *Transaction, ID string, pkey string, wg *sync.WaitGroup) ([]byte, error) {
	if wg != nil {
		defer wg.Done()
	}
	jsObj, err := json.Marshal(txn)
	if err != nil {
		logging.Logger.Error("Error in serializing the transaction", zap.String("error", err.Error()), zap.Any("transaction", txn))
		return nil, err
	}

	return SendPostRequest(url, jsObj, ID, pkey, nil)
}

// MakeGetRequest make a generic get request. URL should have complete path.
// It allows 200 responses only, returning error for all other, even successful.
func MakeGetRequest(remoteUrl string, result interface{}) (err error) {
	logging.N2n.Info("make GET request", zap.String("url", remoteUrl))

	var (
		client http.Client
		rq     *http.Request
	)

	rq, err = http.NewRequest(http.MethodGet, remoteUrl, nil)
	if err != nil {
		return fmt.Errorf("make GET: can't create HTTP request "+
			"on given URL %q: %v", remoteUrl, err)
	}

	var resp *http.Response
	if resp, err = client.Do(rq); err != nil {
		return fmt.Errorf("make GET: requesting %q: %v", remoteUrl, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("make GET: non-200 response code %d: %s",
			resp.StatusCode, resp.Status)
	}

	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("make GET: decoding response: %v", err)
	}

	return // ok
}

//MakeClientBalanceRequest to get a client's balance
func MakeClientBalanceRequest(clientID string, urls []string, consensus int) (state.Balance, error) {
	//ToDo: This looks a lot like GetTransactionConfirmation. Need code reuse?

	//maxCount := 0
	numSuccess := 0
	numErrs := 0

	var clientState state.State
	var errString string

	for _, sharder := range urls {
		u := fmt.Sprintf("%v/%v%v", sharder, clientBalanceURL, clientID)

		logging.N2n.Info("Running GetClientBalance on", zap.String("url", u))

		response, err := http.Get(u)
		if err != nil {
			logging.N2n.Error("Error getting response for sc rest api", zap.Any("error", err))
			numErrs++
			errString = errString + sharder + ":" + err.Error()
			continue
		}

		if response.StatusCode != 200 {
			logging.N2n.Error("Error getting response from", zap.String("URL", sharder), zap.Any("response Status", response.StatusCode))
			numErrs++
			errString = errString + sharder + ": response_code: " + strconv.Itoa(response.StatusCode)
			continue
		}

		d := json.NewDecoder(response.Body)
		d.UseNumber()
		err = d.Decode(&clientState)
		response.Body.Close()
		if err != nil {
			logging.Logger.Error("Error unmarshalling response", zap.Any("error", err))
			numErrs++
			errString = errString + sharder + ":" + err.Error()
			continue
		}

		numSuccess++
	}

	if numSuccess+numErrs == 0 {
		return 0, common.NewError("req_not_run", "Could not run the request") //why???
	}

	sr := int(math.Ceil((float64(numSuccess) * 100) / float64(numSuccess+numErrs)))

	// We've at least one success and success rate sr is at least same as consensus
	if numSuccess > 0 && sr >= consensus {
		return clientState.Balance, nil
	} else if numSuccess > 0 {
		//we had some successes, but not sufficient to reach consensus
		logging.Logger.Error("Error Getting consensus", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return 0, common.NewError("err_getting_consensus", errString)
	} else if numErrs > 0 {
		//We have received only errors
		logging.Logger.Error("Error running the request", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return 0, common.NewError("err_running_req", errString)
	}

	//this should never happen
	return 0, common.NewError("unknown_err", "Not able to run the request. unknown reason")
}

//MakeSCRestAPICall for smart contract REST API Call
func MakeSCRestAPICall(ctx context.Context, scAddress string, relativePath string, params map[string]string, urls []string, entity util.Serializable, consensus int) error {

	//ToDo: This looks a lot like GetTransactionConfirmation. Need code reuse?
	var (
		numSuccess int32
		numErrs    int32
		errStringC = make(chan string, len(urls))
		respDataC  = make(chan []byte, len(urls))
	)

	// get the entity type
	entityType := reflect.TypeOf(entity).Elem()

	//normally this goes to sharders
	wg := &sync.WaitGroup{}
	for _, sharder := range urls {
		wg.Add(1)
		go func(sharderURL string) {
			defer wg.Done()
			urlString := fmt.Sprintf("%v/%v%v%v", sharderURL, scRestAPIURL, scAddress, relativePath)
			logging.N2n.Info("Running SCRestAPI on", zap.String("urlString", urlString))
			urlObj, _ := url.Parse(urlString)
			q := urlObj.Query()
			for k, v := range params {
				q.Add(k, v)
			}
			urlObj.RawQuery = q.Encode()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlObj.String(), nil)
			if err != nil {
				logging.N2n.Error("SCRestAPI - create http request with context failed", zap.Error(err))
			}

			rsp, err := httpClient.Do(req)
			if err != nil {
				logging.N2n.Error("SCRestAPI - error getting response for sc rest api", zap.Any("error", err))
				atomic.AddInt32(&numErrs, 1)
				errStringC <- sharderURL + ":" + err.Error()
				return
			}
			defer rsp.Body.Close()
			if rsp.StatusCode != 200 {
				logging.N2n.Error("SCRestAPI Error getting response from", zap.String("URL", sharderURL), zap.Any("response Status", rsp.StatusCode))
				atomic.AddInt32(&numErrs, 1)
				errStringC <- sharderURL + ": response_code: " + strconv.Itoa(rsp.StatusCode)
				return
			}

			bodyBytes, err := ioutil.ReadAll(rsp.Body)
			if err != nil {
				logging.Logger.Error("SCRestAPI - failed to read body response", zap.String("URL", sharderURL), zap.Any("error", err))
			}
			newEntity := reflect.New(entityType).Interface().(util.Serializable)
			if err := newEntity.Decode(bodyBytes); err != nil {
				logging.Logger.Error("SCRestAPI - error unmarshalling response", zap.Any("error", err))
				atomic.AddInt32(&numErrs, 1)
				errStringC <- sharderURL + ":" + err.Error()
				return
			}
			respDataC <- bodyBytes
			atomic.AddInt32(&numSuccess, 1)

			/*
				Todo: Incorporate hash verification
				hashBytes := h.Sum(nil)
				hash := hex.EncodeToString(hashBytes)
				responses[hash]++
				if responses[hash] > maxCount {
					maxCount = responses[hash]
					retObj = entity
				}
			*/
		}(sharder)
	}

	wg.Wait()
	close(errStringC)
	close(respDataC)
	errStrs := make([]string, 0, len(urls))
	for s := range errStringC {
		errStrs = append(errStrs, s)
	}

	errStr := strings.Join(errStrs, " ")

	nSuccess := atomic.LoadInt32(&numSuccess)
	nErrs := atomic.LoadInt32(&numErrs)
	logging.Logger.Info("SCRestAPI - sc rest consensus", zap.Any("success", nSuccess))
	if nSuccess+nErrs == 0 {
		return common.NewError("req_not_run", "Could not run the request") //why???
	}
	sr := int(math.Ceil((float64(nSuccess) * 100) / float64(nSuccess+nErrs)))
	// We've at least one success and success rate sr is at least same as consensus
	if nSuccess > 0 && sr >= consensus {
		// choose the first returned entity
		select {
		case data := <-respDataC:
			if err := entity.Decode(data); err != nil {
				logging.Logger.Error("SCRestAPI - decode failed", zap.Error(err))
				return nil
			}
		default:
		}
		return nil
	} else if nSuccess > 0 {
		//we had some successes, but not sufficient to reach consensus
		logging.Logger.Error("SCRestAPI - error Getting consensus",
			zap.Int32("Success", nSuccess),
			zap.Int32("Errs", nErrs),
			zap.Int("consensus", consensus))
		return common.NewError("err_getting_consensus", errStr)
	} else if nErrs > 0 {
		//We have received only errors
		logging.Logger.Error("SCRestAPI - error running the request",
			zap.Int32("Success", nSuccess),
			zap.Int32("Errs", nErrs),
			zap.Int("consensus", consensus))
		return common.NewError("err_running_req", errStr)
	}
	//this should never happen
	return common.NewError("unknown_err", "Not able to run the request. unknown reason")
}

// MakeSCRestAPICall for smart contract REST API Call
func GetBlockSummaryCall(urls []string, consensus int, magicBlock bool) (*block.BlockSummary, error) {

	//ToDo: This looks a lot like GetTransactionConfirmation. Need code reuse?
	//responses := make(map[string]int)
	var retObj interface{}
	//maxCount := 0
	numSuccess := 0
	numErrs := 0
	var errString string
	summary := &block.BlockSummary{}
	// magicBlock := block.NewMagicBlock()

	//normally this goes to sharders
	for _, sharder := range urls {
		var blockUrl string
		if magicBlock {
			blockUrl = magicBlockURL
		} else {
			blockUrl = finalizeBlockURL
		}
		response, err := httpClient.Get(fmt.Sprintf("%v/%v", sharder, blockUrl))
		if err != nil {
			logging.N2n.Error("Error getting response for sc rest api", zap.Any("error", err))
			numErrs++
			errString = errString + sharder + ":" + err.Error()
		} else {
			if response.StatusCode != 200 {
				logging.N2n.Error("Error getting response from", zap.String("URL", sharder), zap.Any("response Status", response.StatusCode))
				numErrs++
				errString = errString + sharder + ": response_code: " + strconv.Itoa(response.StatusCode)
				response.Body.Close()
				continue
			}
			bodyBytes, err := ioutil.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				logging.Logger.Error("Failed to read body response", zap.String("URL", sharder), zap.Any("error", err))
			}
			summary.Decode(bodyBytes)
			logging.Logger.Info("get magic block -- entity", zap.Any("summary", summary))
			// logging.Logger.Info("get magic block -- entity", zap.Any("magic_block", entity), zap.Any("string of magic block", string(bodyBytes)))
			if err != nil {
				logging.Logger.Error("Error unmarshalling response", zap.Any("error", err))
				numErrs++
				errString = errString + sharder + ":" + err.Error()
				continue
			}
			retObj = summary
			numSuccess++
		}
	}

	if numSuccess+numErrs == 0 {
		return nil, common.NewError("req_not_run", "Could not run the request") //why???

	}
	sr := int(math.Ceil((float64(numSuccess) * 100) / float64(numSuccess+numErrs)))
	// We've at least one success and success rate sr is at least same as consensus
	if numSuccess > 0 && sr >= consensus {
		if retObj != nil {
			return summary, nil
		}
		return nil, common.NewError("err_getting_resp", errString)
	} else if numSuccess > 0 {
		//we had some successes, but not sufficient to reach consensus
		logging.Logger.Error("Error Getting consensus", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return nil, common.NewError("err_getting_consensus", errString)
	} else if numErrs > 0 {
		//We have received only errors
		logging.Logger.Error("Error running the request", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return nil, common.NewError("err_running_req", errString)
	}
	//this should never happen
	return nil, common.NewError("unknown_err", "Not able to run the request. unknown reason")

}

// TODO: Don't use this function. It doesn't to block validation. Just used for testing.
func FetchMagicBlockFromSharders(ctx context.Context, sharderURLs []string, number int64) (*block.Block, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	done := false
	recv := make(chan *block.Block)

	for _, url := range sharderURLs {
		go func(url string) {
			resp, err := httpClient.Get(url)
			if done || err != nil || resp.StatusCode != http.StatusOK {
				return
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if done || err != nil {
				return
			}
			b := datastore.GetEntityMetadata("block").Instance().(*block.Block)
			err = b.Decode(body)
			if done || err != nil {
				return
			}
			if b.MagicBlock != nil &&
				b.MagicBlockNumber == number {
				select {
				case recv <- b:
				default:
				}
			}
		}(fmt.Sprintf("%v/%v%v", url, specificMagicBlockURL, number))
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to fetch MB from sharders: n=%d, err=%v", number, ctx.Err())
		case b := <-recv:
			done = true
			return b, nil
		}
	}
}

//GetMagicBlockCall for smart contract to get magic block
func GetMagicBlockCall(urls []string, magicBlockNumber int64, consensus int) (*block.Block, error) {
	var retObj interface{}
	numSuccess := 0
	numErrs := 0
	var errString string
	timeoutRetry := time.Millisecond * 500
	receivedBlock := datastore.GetEntityMetadata("block").Instance().(*block.Block)
	receivedBlock.MagicBlock = block.NewMagicBlock()

	for _, sharder := range urls {
		u := fmt.Sprintf("%v/%v%v", sharder, specificMagicBlockURL, strconv.FormatInt(magicBlockNumber, 10))

		retried := 0
		var response *http.Response
		var err error
		for {
			response, err = httpClient.Get(u)
			if err != nil || retried >= 4 || response.StatusCode != http.StatusTooManyRequests {
				break
			}
			response.Body.Close()
			logging.N2n.Warn("attempt to retry the request",
				zap.Any("response Status", response.StatusCode),
				zap.Any("response Status text", response.Status), zap.String("URL", u),
				zap.Any("retried", retried+1))
			time.Sleep(timeoutRetry)
			retried++
		}

		if err != nil {
			logging.N2n.Error("Error getting response for sc rest api", zap.Any("error", err))
			numErrs++
			errString = errString + sharder + ":" + err.Error()
		} else {
			if response.StatusCode != 200 {
				logging.N2n.Error("Error getting response from", zap.String("URL", u),
					zap.Any("response Status", response.StatusCode),
					zap.Any("response Status text", response.Status))
				numErrs++
				errString = errString + sharder + ": response_code: " + strconv.Itoa(response.StatusCode)
				response.Body.Close()
				continue
			}
			bodyBytes, err := ioutil.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				logging.Logger.Error("Failed to read body response", zap.String("URL", sharder), zap.Any("error", err))
			}
			err = receivedBlock.Decode(bodyBytes)
			if err != nil {
				logging.Logger.Error("failed to decode block", zap.Any("error", err))
			}

			if err != nil {
				logging.Logger.Error("Error unmarshalling response", zap.Any("error", err))
				numErrs++
				errString = errString + sharder + ":" + err.Error()
				continue
			}
			retObj = receivedBlock
			numSuccess++
		}
	}

	if numSuccess+numErrs == 0 {
		return nil, common.NewError("req_not_run", "Could not run the request")
	}
	sr := int(math.Ceil((float64(numSuccess) * 100) / float64(numSuccess+numErrs)))
	if numSuccess > 0 && sr >= consensus {
		if retObj != nil {
			return receivedBlock, nil
		}
		return nil, common.NewError("err_getting_resp", errString)
	} else if numSuccess > 0 {
		logging.Logger.Error("Error Getting consensus", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return nil, common.NewError("err_getting_consensus", errString)
	} else if numErrs > 0 {
		logging.Logger.Error("Error running the request", zap.Int("Success", numSuccess), zap.Int("Errs", numErrs), zap.Int("consensus", consensus))
		return nil, common.NewError("err_running_req", errString)
	}
	return nil, common.NewError("unknown_err", "Not able to run the request. unknown reason")

}

func SendSmartContractTxn(txn *Transaction, address string, value, fee int64, scData *SmartContractTxnData, minerUrls []string) error {
	txn.ToClientID = address
	txn.Value = value
	txn.Fee = fee
	txn.TransactionType = TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		logging.Logger.Error("Returning error", zap.Error(err))
		return err
	}
	txn.TransactionData = string(txnBytes)

	signer := func(hash string) (string, error) {
		return node.Self.Sign(hash)
	}

	err = txn.ComputeHashAndSign(signer)
	if err != nil {
		logging.Logger.Info("Signing Failed during registering miner to the mining network", zap.Error(err))
		return err
	}
	SendTransaction(txn, minerUrls, node.Self.Underlying().GetKey(),
		node.Self.Underlying().PublicKey)
	return nil
}
