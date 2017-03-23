import OpenOPC
import socket
import clearblade
from clearblade import auth
from clearblade import Client
from clearblade import Messaging
from time import sleep

gateway = "127.0.0.1"
opcHost = "127.0.0.1"
auth = auth.Auth()

userClient = Client.UserClient("acc19f800bd2d69febc2c6869309", "ACC19F800BC4FAAFECEDF1EAAB4F", "opc_machine@clearblade.com", "clearblade", "http://192.168.0.82:9000")
auth.Authenticate(userClient)

message = Messaging.Messaging(userClient)
message.printValue()
message.InitializeMQTT()

opcServer = 'Matrikon.OPC.Simulation'

taglist = ['Triangle Waves.UInt2'] 

print "Connecting to OPC gateway"

opcConnect = OpenOPC.open_client(gateway)
opcConnect.connect(opcServer, opcHost)

print "Connected!!"

tags = opcConnect.read(taglist)
#opcConnect.close()

for i in range(len(tags)):
    (tag, reading, condition, time) = tags[i]
    print "%s    %s     %s     %s"%(tag, reading, condition, time)
	
'''s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
host = ''
port = 12345
s.bind((host,port))'''

#s.listen(5)
#while True:
    #c, addr = s.accept()
    #t = c.recv(1024)
count = 0
while 1:
	def onMessageCallback(client, obj, msg):
		print "Payload: "+msg.payload
		print "Machine temperature exceeds threshold. Sending message to shutdown and reset the temperature"
		dataToBeSent = msg.payload.split(',')
		print dataToBeSent[0]
		print dataToBeSent[1]
		for i in range(10):
			result = opcConnect.write((dataToBeSent[0], dataToBeSent[1]))
			sleep(1)
	if count == 0:	
		message.subscribe("StatusCodeSent", 1, onMessageCallback)
	count += 1
	tags = opcConnect.read(taglist)
	(tag, reading, condition, time) = tags[i]
	#print "%s    %s     %s     %s"%(tag, reading, condition, time)
	message.publishMessage("MachineCodes", reading, 1)
		
'''while True:
	pass
    #break'''
opcConnect.close()	