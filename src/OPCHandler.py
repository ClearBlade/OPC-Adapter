import SocketServer
import OpenOPC

gateway = "10.211.55.3"
opcHost = "10.211.55.3"
opcServer = 'Matrikon.OPC.Simulation'

class OPCHandler(SocketServer.BaseRequestHandler):
    def handle(self):
        # self.request is the TCP socket connected to the client
        self.data = self.request.recv(1024).strip()
        # print "{} wrote:".format(self.client_address[0])
        print self.data
        taglist = [self.data]

        opcConnect = OpenOPC.open_client(gateway)
        opcConnect.connect(opcServer, opcHost)

        tags = opcConnect.read(taglist)
        opcConnect.close()

        for i in range(len(tags)):
            (tag, reading, condition, time) = tags[i]
            print "%s    %s     %s     %s"%(tag, reading, condition, time)
            self.request.sendall(str(tags[i]))

if __name__ == "__main__":
    HOST, PORT = "0.0.0.0", 8999

    server = SocketServer.TCPServer((HOST, PORT), OPCHandler)
    server.serve_forever()
