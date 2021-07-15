(window.webpackJsonp=window.webpackJsonp||[]).push([[82],{650:function(t,e,n){"use strict";n.r(e);var s=n(0),a=Object(s.a)({},(function(){var t=this,e=t.$createElement,n=t._self._c||e;return n("ContentSlotsDistributor",{attrs:{"slot-key":t.$parent.slotKey}},[n("h1",{attrs:{id:"joiningtestnet"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#joiningtestnet"}},[t._v("#")]),t._v(" JoiningTestnet")]),t._v(" "),n("p",[t._v("This document outlines the steps to join the public testnet")]),t._v(" "),n("h2",{attrs:{id:"steps"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#steps"}},[t._v("#")]),t._v(" Steps")]),t._v(" "),n("ol",[n("li",[n("p",[t._v("Install the Ethermint binaries (ethermintd & ethermint cli)")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"Z2l0IGNsb25lIGh0dHBzOi8vZ2l0aHViLmNvbS90aGFyc2lzL2V0aGVybWludApjZCBldGhlcm1pbnQKZ2l0IGNoZWNrb3V0IHYwLjQuMQptYWtlIGluc3RhbGwK"}})],1),t._v(" "),n("li",[n("p",[t._v("Create an Ethermint account")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"ZXRoZXJtaW50Y2xpIGtleXMgYWRkICZsdDtrZXluYW1lJmd0Owo="}})],1),t._v(" "),n("li",[n("p",[t._v("Copy genesis file")]),t._v(" "),n("p",[t._v("Follow this "),n("a",{attrs:{href:"https://gist.github.com/araskachoi/43f86f3edff23729b817e8b0bb86295a",target:"_blank",rel:"noopener noreferrer"}},[t._v("link"),n("OutboundLink")],1),t._v(" and copy it over to the directory ~/.ethermintd/config/genesis.json")])]),t._v(" "),n("li",[n("p",[t._v("Add peers")]),t._v(" "),n("p",[t._v("Edit the file located in ~/.ethermintd/config/config.toml and edit line 350 (persistent_peers) to the following")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"toml",base64:"JnF1b3Q7MDVhYTY1ODdmMDdhMGM2YTlhODIxM2YwMTM4YzRhNzZkNDc2NDE4YUAxOC4yMDQuMjA2LjE3OToyNjY1NiwxM2Q0YTFjMTZkMWY0Mjc5ODhiN2M0OTliNmQxNTA3MjZhYWYzYWEwQDMuODYuMTA0LjI1MToyNjY1NixhMDBkYjc0OWZhNTFlNDg1YzgzNzYyNzZkMjlkNTk5MjU4MDUyZjNlQDU0LjIxMC4yNDYuMTY1OjI2NjU2JnF1b3Q7Cg=="}})],1),t._v(" "),n("li",[n("p",[t._v("Validate genesis and start the Ethermint network")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"ZXRoZXJtaW50ZCB2YWxpZGF0ZS1nZW5lc2lzCgpldGhlcm1pbnRkIHN0YXJ0IC0tcHJ1bmluZz1ub3RoaW5nIC0tcnBjLnVuc2FmZSAtLWxvZ19sZXZlbCAmcXVvdDttYWluOmluZm8sc3RhdGU6aW5mbyxtZW1wb29sOmluZm8mcXVvdDsgLS10cmFjZQo="}}),t._v(" "),n("p",[t._v("(we recommend running the command in the background for convenience)")])],1),t._v(" "),n("li",[n("p",[t._v("Start the RPC server")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"ZXRoZXJtaW50Y2xpIHJlc3Qtc2VydmVyIC0tbGFkZHIgJnF1b3Q7dGNwOi8vbG9jYWxob3N0Ojg1NDUmcXVvdDsgLS11bmxvY2sta2V5ICRLRVkgLS1jaGFpbi1pZCBldGhlcm1pbnR0ZXN0bmV0LTc3NyAtLXRyYWNlIC0tcnBjLWFwaSAmcXVvdDt3ZWIzLGV0aCxuZXQmcXVvdDsK"}}),t._v(" "),n("p",[t._v("where "),n("code",[t._v("$KEY")]),t._v(" is the key name that was used in step 2.\n(we recommend running the command in the background for convenience)")])],1),t._v(" "),n("li",[n("p",[t._v("Request funds from the faucet")]),t._v(" "),n("p",[t._v("You will need to know the Ethereum hex address, and it can be found with the following command:")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"Y3VybCAtWCBQT1NUIC0tZGF0YSAneyZxdW90O2pzb25ycGMmcXVvdDs6JnF1b3Q7Mi4wJnF1b3Q7LCZxdW90O21ldGhvZCZxdW90OzomcXVvdDtldGhfYWNjb3VudHMmcXVvdDssJnF1b3Q7cGFyYW1zJnF1b3Q7OltdLCZxdW90O2lkJnF1b3Q7OjF9JyAtSCAmcXVvdDtDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL2pzb24mcXVvdDsgaHR0cDovL2xvY2FsaG9zdDo4NTQ1Cg=="}}),t._v(" "),n("p",[t._v("Using the output of the above command, you will then send the command with your valid Ethereum address")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"Y3VybCAtLWhlYWRlciAmcXVvdDtDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL2pzb24mcXVvdDsgLS1yZXF1ZXN0IFBPU1QgLS1kYXRhICd7JnF1b3Q7YWRkcmVzcyZxdW90OzomcXVvdDsweFlvdXJFdGhlcmV1bUhleEFkZHJlc3MmcXVvdDt9JyAzLjk1LjIxLjkxOjMwMDAK"}})],1)]),t._v(" "),n("h2",{attrs:{id:"public-testnet-node-rpc-endpoints"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#public-testnet-node-rpc-endpoints"}},[t._v("#")]),t._v(" Public Testnet Node RPC Endpoints")]),t._v(" "),n("ul",[n("li",[n("strong",[t._v("Node0")]),t._v(": "),n("code",[t._v("54.210.246.165:8545")])]),t._v(" "),n("li",[n("strong",[t._v("Node1")]),t._v(": "),n("code",[t._v("3.86.104.251:8545")])]),t._v(" "),n("li",[n("strong",[t._v("Node2")]),t._v(": "),n("code",[t._v("18.204.206.179:8545")])])]),t._v(" "),n("p",[t._v("example:")]),t._v(" "),n("tm-code-block",{staticClass:"codeblock",attrs:{language:"bash",base64:"Y3VybCAtWCBQT1NUIC0tZGF0YSAneyZxdW90O2pzb25ycGMmcXVvdDs6JnF1b3Q7Mi4wJnF1b3Q7LCZxdW90O21ldGhvZCZxdW90OzomcXVvdDtldGhfY2hhaW5JZCZxdW90OywmcXVvdDtwYXJhbXMmcXVvdDs6W10sJnF1b3Q7aWQmcXVvdDs6MX0nIC1IICZxdW90O0NvbnRlbnQtVHlwZTogYXBwbGljYXRpb24vanNvbiZxdW90OyA1NC4yMTAuMjQ2LjE2NTo4NTQ1Cg=="}})],1)}),[],!1,null,null,null);e.default=a.exports}}]);