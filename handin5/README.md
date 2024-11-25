Setting up the auction:

- Step 1
  First navigate to the handin5/server folder
  Set up the auction server nodes by running the following commands in your terminal
  Primary server: "go run server.go --isPrimary=true --Port=1337"
  Backup server: "go run server.go --isPrimary=false --Port=1338"

- Step 2
  First navigate to the handin5/client folder
  Set up the auction clients by running the following commands n amount of times (depending on the number of clients you want to participate in the auction)
  Client: "go run client.go"

- Step 3
  Once you have started the primary- and backup server nodes and the clients, you can now start the auction.
  Start the auction by placing a bid: "bid <amount>". Replace <amount> with the amount of money you would like to bid.
  You can run the command "help" in the client terminal to show all the commands you can run

If you would like to exit the auction, simply press "ctrl + c" and the client will exit gracefully


NOTE: To simulate a server crash, you can press "ctrl + c" in the primary server terminal