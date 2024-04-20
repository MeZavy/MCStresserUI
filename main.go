import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/bot/basic"
	"github.com/Tnze/go-mc/chat"
)

var (
	htmlContent = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Minecraft Bot Control Panel</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            background-color: #222;
            color: #fff;
            margin: 0;
            padding: 0;
            display: flex;
            flex-direction: column;
            align-items: center;
            min-height: 100vh;
        }
        .container {
            text-align: center;
        }
        h1 {
            margin-bottom: 20px;
        }
        input[type="text"] {
            padding: 8px;
            margin: 5px;
            border-radius: 5px;
            border: none;
            background-color: #444;
            color: #fff;
            width: 200px;
        }
        .button {
            padding: 10px 20px;
            background-color: #444;
            color: #fff;
            border: none;
            cursor: pointer;
            outline: none;
            border-radius: 5px;
            transition: background-color 0.3s ease;
            box-shadow: 0px 2px 5px rgba(0, 0, 0, 0.3);
        }
        .button:hover {
            background-color: #555;
        }
        #output {
            width: 80%;
            max-width: 600px;
            background-color: #333;
            padding: 10px;
            margin-top: 20px;
            border-radius: 5px;
            overflow-y: auto;
            height: 200px;
            line-height: 1.5;
            margin: auto; /* Center output box */
        }
        #console {
            width: 80%;
            max-width: 600px;
            background-color: #333;
            padding: 10px;
            margin-top: 20px;
            border-radius: 5px;
            overflow-y: auto;
            height: 200px;
            line-height: 1.5;
            margin: auto; /* Center console box */
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Minecraft Bot Control Panel</h1>
        <label for="protocol">Protocol Version:</label><br>
        <input type="text" id="protocol" value="763"><br>
        <label for="address">Server Address:</label><br>
        <input type="text" id="address" value="127.0.0.1"><br>
        <label for="username">Username:</label><br>
        <input type="text" id="username" value="FifthColumn"><br>
        <label for="number">Number of Clients:</label><br>
        <input type="text" id="number" value="2048"><br>
        <button class="button" onclick="startBots()">Start Bots</button>
        <button class="button" onclick="stopBots()">Stop Bots</button>
        <div id="output"></div>
        <textarea id="console" readonly></textarea>
    </div>
    <script>
        function startBots() {
            var protocol = document.getElementById('protocol').value;
            var address = document.getElementById('address').value;
            var username = document.getElementById('username').value;
            var number = document.getElementById('number').value;
            appendOutput('Starting bots...');
            fetch('/start?protocol=' + protocol + '&address=' + address + '&username=' + username + '&number=' + number)
                .then(response => response.text())
                .then(data => appendOutput(data))
                .catch(error => appendOutput('Error starting bots: ' + error));
        }

        function stopBots() {
            appendOutput('Stopping bots...');
            fetch('/stop')
                .then(response => response.text())
                .then(data => appendOutput(data))
                .catch(error => appendOutput('Error stopping bots: ' + error));
        }

        function appendOutput(message) {
            var outputDiv = document.getElementById('output');
            outputDiv.innerHTML += message + '<br>';
            outputDiv.scrollTop = outputDiv.scrollHeight;
        }

        function appendToConsole(message) {
            var consoleTextarea = document.getElementById('console');
            consoleTextarea.value += message + '\n';
            consoleTextarea.scrollTop = consoleTextarea.scrollHeight;
        }
    </script>
</body>
</html>
`

	running       bool
	stopRequested bool
	stopMutex     sync.Mutex
	botsWG        sync.WaitGroup
)

func main() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/start", handleStart)
	http.HandleFunc("/stop", handleStop)

	log.Println("Starting server at :8080...")
	go logToConsole()

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, htmlContent)
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	if running {
		fmt.Fprint(w, "Bots are already running.")
		return
	}

	protocol := r.URL.Query().Get("protocol")
	address := r.URL.Query().Get("address")
	username := r.URL.Query().Get("username")
	numberStr := r.URL.Query().Get("number")

	p, err := strconv.Atoi(protocol)
	if err != nil {
		fmt.Fprint(w, "Invalid protocol version.")
		return
	}

	num, err := strconv.Atoi(numberStr)
	if err != nil {
		fmt.Fprint(w, "Invalid number of clients.")
		return
	}

	startBots(address, username, p, num)
	fmt.Fprint(w, "Bots started successfully.")
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	stopMutex.Lock()
	stopRequested = true
	stopMutex.Unlock()
	botsWG.Wait()
	fmt.Fprint(w, "Bot stopping is in the works DOESNT WORK YET!")
}

func startBots(address, username string, protocol, number int) {
	stopMutex.Lock()
	stopRequested = false
	stopMutex.Unlock()

	botsWG.Add(number)
	for i := 0; i < number; i++ {
		go func(i int) {
			for {
				if stopRequested {
					break
				}

				ind := newIndividual(i, username)
				ind.run(address, protocol)
				time.Sleep(time.Second)
			}
			botsWG.Done()
		}(i)
	}
}

type individual struct {
	id     int
	client *bot.Client
	player *basic.Player
}

func newIndividual(id int, username string) *individual {
	i := &individual{
		id: id,
	}
	i.client = bot.NewClient()
	i.client.Auth = bot.Auth{
		Name: username + strconv.Itoa(id),
	}
	i.player = basic.NewPlayer(i.client, basic.DefaultSettings, basic.EventsListener{
		GameStart:  i.onGameStart,
		Disconnect: onDisconnect,
	})
	return i
}

func (i *individual) run(address string, protocolVersion int) {
	err := i.client.JoinServer(address, protocolVersion)
	if err != nil {
		log.Printf("[%d]Login fail: %v", i.id, err)
		return
	}
	defer i.client.Close()
	log.Printf("[%d]Login success", i.id)

	if err = i.client.HandleGame(); err != nil {
		log.Printf("[%d]HandleGame error: %v", i.id, err)
		return
	}
}

func (i *individual) onGameStart() error {
	log.Printf("[%d]Game start", i.id)
	return nil
}

func onDisconnect(reason chat.Message) error {
	log.Printf("Disconnect Reason: %s", reason)
	return nil
}

func logToConsole() {
	for {
		time.Sleep(time.Second)
		log.Println("MCStress UI by MeZavy!")
	}
}
