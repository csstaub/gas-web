// This file is originally from HewlettPackard/gas.
// --
// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"go/ast"

	gas "github.com/HewlettPackard/gas/core"
	"github.com/HewlettPackard/gas/rules"
)

type ruleInfo struct {
	description string
	build       func(map[string]interface{}) (gas.Rule, ast.Node)
}

var allRules = map[string]ruleInfo{
	// misc
	"G101": ruleInfo{"Look for hardcoded credentials", rules.NewHardcodedCredentials},
	"G102": ruleInfo{"Bind to all interfaces", rules.NewBindsToAllNetworkInterfaces},
	"G103": ruleInfo{"Audit the use of unsafe block", rules.NewUsingUnsafe},
	"G104": ruleInfo{"Audit errors not checked", rules.NewTemplateCheck},

	// injection
	"G201": ruleInfo{"SQL query construction using format string", rules.NewSqlStrFormat},
	"G202": ruleInfo{"SQL query construction using string concatenation", rules.NewSqlStrConcat},
	"G203": ruleInfo{"Use of unescaped data in HTML templates", rules.NewTemplateCheck},
	"G204": ruleInfo{"Audit use of command execution", rules.NewSubproc},

	// filesystem
	"G301": ruleInfo{"Poor file permissions used when creating a directory", rules.NewMkdirPerms},
	"G302": ruleInfo{"Poor file permisions used with chmod", rules.NewChmodPerms},
	"G303": ruleInfo{"Creating tempfile using a predictable path", rules.NewBadTempFile},

	// crypto
	"G401": ruleInfo{"Detect the usage of DES, RC4, or MD5", rules.NewUsesWeakCryptography},
	"G402": ruleInfo{"Look for bad TLS connection settings", rules.NewIntermediateTlsCheck},
	"G403": ruleInfo{"Ensure minimum RSA key length of 2048 bits", rules.NewWeakKeyStrength},
	"G404": ruleInfo{"Insecure random number source (rand)", rules.NewWeakRandCheck},

	// blacklist
	"G501": ruleInfo{"Import blacklist: crypto/md5", rules.NewBlacklist_crypto_md5},
	"G502": ruleInfo{"Import blacklist: crypto/des", rules.NewBlacklist_crypto_des},
	"G503": ruleInfo{"Import blacklist: crypto/rc4", rules.NewBlacklist_crypto_rc4},
	"G504": ruleInfo{"Import blacklist: net/http/cgi", rules.NewBlacklist_net_http_cgi},
}

func addRules(analyzer *gas.Analyzer, conf map[string]interface{}) {
	for _, v := range allRules {
		analyzer.AddRule(v.build(conf))
	}
}
