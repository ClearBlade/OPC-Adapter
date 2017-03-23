package dukdukgo

var opcjs = `
function opc(){
	_this = this;

	opc.sendConnectionRequest = function(ip_port, conn_type, callback){
		log("Trying to connect");
		var resp = connect(ip_port, conn_type);
		if (resp.error === undefined){
			this.execute(false, "Connected to server",callback);
		}else{
			this.execute(true,response.error,callback);
		}
	}
}

return this;
}  
`