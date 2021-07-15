(window.webpackJsonp=window.webpackJsonp||[]).push([[97],{605:function(g,I,t){"use strict";t.r(I);var l=t(0),C=Object(l.a)({},(function(){var g=this,I=g.$createElement,t=g._self._c||I;return t("ContentSlotsDistributor",{attrs:{"slot-key":g.$parent.slotKey}},[t("h1",{attrs:{id:"evm"}},[t("a",{staticClass:"header-anchor",attrs:{href:"#evm"}},[g._v("#")]),g._v(" "),t("code",[g._v("evm")])]),g._v(" "),t("h2",{attrs:{id:"abstract"}},[t("a",{staticClass:"header-anchor",attrs:{href:"#abstract"}},[g._v("#")]),g._v(" Abstract")]),g._v(" "),t("p",[g._v("This document defines the specification of the Ethereum Virtual Machine (EVM) as a Cosmos SDK module.")]),g._v(" "),t("h2",{attrs:{id:"contents"}},[t("a",{staticClass:"header-anchor",attrs:{href:"#contents"}},[g._v("#")]),g._v(" Contents")]),g._v(" "),t("ol",[t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/01_concepts.html"}},[g._v("Concepts")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/02_state.html"}},[g._v("State")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/03_state_transitions.html"}},[g._v("State Transitions")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/04_messages.html"}},[g._v("Messages")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/05_abci.html"}},[g._v("ABCI")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/06_events.html"}},[g._v("Events")])],1)]),g._v(" "),t("li",[t("strong",[t("RouterLink",{attrs:{to:"/modules/evm/07_params.html"}},[g._v("Parameters")])],1)])]),g._v(" "),t("h2",{attrs:{id:"module-architecture"}},[t("a",{staticClass:"header-anchor",attrs:{href:"#module-architecture"}},[g._v("#")]),g._v(" Module Architecture")]),g._v(" "),t("blockquote",[t("p",[t("strong",[g._v("NOTE:")]),g._v(": If you're not familiar with the overall module structure from\nthe SDK modules, please check this "),t("a",{attrs:{href:"https://docs.cosmos.network/master/building-modules/structure.html",target:"_blank",rel:"noopener noreferrer"}},[g._v("document"),t("OutboundLink")],1),g._v(" as\nprerequisite reading.")])]),g._v(" "),t("tm-code-block",{staticClass:"codeblock",attrs:{language:"shell",base64:"ZXZtLwrilJzilIDilIAgY2xpZW50CuKUgiAgIOKUlOKUgOKUgCBjbGkK4pSCICAgICAgIOKUnOKUgOKUgCBxdWVyeS5nbyAgICAgICMgQ0xJIHF1ZXJ5IGNvbW1hbmRzIGZvciB0aGUgbW9kdWxlCuKUgiAgICDCoMKgIOKUlOKUgOKUgCB0eC5nbyAgICAgICAgICMgQ0xJIHRyYW5zYWN0aW9uIGNvbW1hbmRzIGZvciB0aGUgbW9kdWxlCuKUnOKUgOKUgCBrZWVwZXIK4pSCICAg4pSc4pSA4pSAIGtlZXBlci5nbyAgICAgICAgICMgQUJDSSBCZWdpbkJsb2NrIGFuZCBFbmRCbG9jayBsb2dpYwrilIIgICDilJzilIDilIAga2VlcGVyLmdvICAgICAgICAgIyBTdG9yZSBrZWVwZXIgdGhhdCBoYW5kbGVzIHRoZSBidXNpbmVzcyBsb2dpYyBvZiB0aGUgbW9kdWxlIGFuZCBoYXMgYWNjZXNzIHRvIGEgc3BlY2lmaWMgc3VidHJlZSBvZiB0aGUgc3RhdGUgdHJlZS4K4pSCICAg4pSc4pSA4pSAIHBhcmFtcy5nbyAgICAgICAgICMgUGFyYW1ldGVyIGdldHRlciBhbmQgc2V0dGVyCuKUgiAgIOKUnOKUgOKUgCBxdWVyaWVyLmdvICAgICAgICAjIFN0YXRlIHF1ZXJ5IGZ1bmN0aW9ucwrilIIgICDilJTilIDilIAgc3RhdGVkYi5nbyAgICAgICAgIyBGdW5jdGlvbnMgZnJvbSB0eXBlcy9zdGF0ZWRiIHdpdGggYSBwYXNzZWQgaW4gc2RrLkNvbnRleHQK4pSc4pSA4pSAIHR5cGVzCuKUgsKgwqAg4pSc4pSA4pSAIGNoYWluX2NvbmZpZy5nbwrilILCoMKgIOKUnOKUgOKUgCBjb2RlYy5nbyAgICAgICAgICAjIFR5cGUgcmVnaXN0cmF0aW9uIGZvciBlbmNvZGluZwrilILCoMKgIOKUnOKUgOKUgCBlcnJvcnMuZ28gICAgICAgICAjIE1vZHVsZS1zcGVjaWZpYyBlcnJvcnMK4pSCwqDCoCDilJzilIDilIAgZXZlbnRzLmdvICAgICAgICAgIyBFdmVudHMgZXhwb3NlZCB0byB0aGUgVGVuZGVybWludCBQdWJTdWIvV2Vic29ja2V0CuKUgsKgwqAg4pSc4pSA4pSAIGdlbmVzaXMuZ28gICAgICAgICMgR2VuZXNpcyBzdGF0ZSBmb3IgdGhlIG1vZHVsZQrilILCoMKgIOKUnOKUgOKUgCBqb3VybmFsLmdvICAgICAgICAjIEV0aGVyZXVtIEpvdXJuYWwgb2Ygc3RhdGUgdHJhbnNpdGlvbnMK4pSCwqDCoCDilJzilIDilIAga2V5cy5nbyAgICAgICAgICAgIyBTdG9yZSBrZXlzIGFuZCB1dGlsaXR5IGZ1bmN0aW9ucwrilILCoMKgIOKUnOKUgOKUgCBsb2dzLmdvICAgICAgICAgICAjIFR5cGVzIGZvciBwZXJzaXN0aW5nIEV0aGVyZXVtIHR4IGxvZ3Mgb24gc3RhdGUgYWZ0ZXIgY2hhaW4gdXBncmFkZXMK4pSCwqDCoCDilJzilIDilIAgbXNnLmdvICAgICAgICAgICAgIyBFVk0gbW9kdWxlIHRyYW5zYWN0aW9uIG1lc3NhZ2VzCuKUgsKgwqAg4pSc4pSA4pSAIHBhcmFtcy5nbyAgICAgICAgICMgTW9kdWxlIHBhcmFtZXRlcnMgdGhhdCBjYW4gYmUgY3VzdG9taXplZCB3aXRoIGdvdmVybmFuY2UgcGFyYW1ldGVyIGNoYW5nZSBwcm9wb3NhbHMK4pSCwqDCoCDilJzilIDilIAgc3RhdGVfb2JqZWN0LmdvICAgIyBFVk0gc3RhdGUgb2JqZWN0CuKUgsKgwqAg4pSc4pSA4pSAIHN0YXRlZGIuZ28gICAgICAgICMgSW1wbGVtZW50YXRpb24gb2YgdGhlIFN0YXRlRGIgaW50ZXJmYWNlCuKUgsKgwqAg4pSc4pSA4pSAIHN0b3JhZ2UuZ28gICAgICAgICMgSW1wbGVtZW50YXRpb24gb2YgdGhlIEV0aGVyZXVtIHN0YXRlIHN0b3JhZ2UgbWFwIHVzaW5nIGFycmF5cyB0byBwcmV2ZW50IG5vbi1kZXRlcm1pbmlzbQrilILCoMKgIOKUlOKUgOKUgCB0eF9kYXRhLmdvICAgICAgICAjIEV0aGVyZXVtIHRyYW5zYWN0aW9uIGRhdGEgdHlwZXMK4pSc4pSA4pSAIGdlbmVzaXMuZ28gICAgICAgICAgICAjIEFCQ0kgSW5pdEdlbmVzaXMgYW5kIEV4cG9ydEdlbmVzaXMgZnVuY3Rpb25hbGl0eQrilJzilIDilIAgaGFuZGxlci5nbyAgICAgICAgICAgICMgTWVzc2FnZSByb3V0aW5nCuKUlOKUgOKUgCBtb2R1bGUuZ28gICAgICAgICAgICAgIyBNb2R1bGUgc2V0dXAgZm9yIHRoZSBtb2R1bGUgbWFuYWdlcgo="}})],1)}),[],!1,null,null,null);I.default=C.exports}}]);