import { decode } from '../../plugins/transport.js';

const { Router } = require('express');
const shelljs = require('shelljs');

const router = Router();

function array_order(x, y) {
    var pre = ['string', 'number', 'bool'];
    if (typeof x !== typeof y)
        return pre.indexOf(typeof y) - pre.indexOf(typeof x);

    if (x === y) return 0;
    else return x > y ? 1 : -1;
}

/* GET users listing. */
router.post('/query', function(req, res, next) {
    console.log('Incoming DNS query');
    if (req && req.body) {
        // Define variables
        let payload = {};
        let data = [];

        // Set payload from request body
        payload = req.body;

        // Decode payload data
        const decoded = decode(payload.data);

        // Server & Records
        const servers = decoded.servers;
        const records = decoded.records;

        records.forEach(function(record) {
            // Create record set
            let rs = {
                record: {
                    fqdn: record.fqdn,
                    type: record.type,
                },
                results: [],
                diff: false,
            };

            // Loop thru all servers to be resolved with
            servers.forEach(function(server) {
                // Use dig for DNS queries
                let query;
                const command = `dig ${record.fqdn} ${record.type} @${server.address} +short`;

                let validation = {
                    status: true,
                    error: null,
                };

                // Check server
                if (
                    !/^[A-Za-z0-9\.\-\_]{1,}\.[A-Za-z0-9\.\-\_]{1,}$/.test(
                        server.address
                    )
                ) {
                    validation = {
                        status: false,
                        error: server.address,
                    };
                }

                // Check record
                if (
                    !/^[A-Za-z0-9\.\-\_]{1,}\.[A-Za-z0-9\.\-\_]{1,}$/.test(
                        record.fqdn
                    )
                ) {
                    validation = {
                        status: false,
                        error: record.fqdn,
                    };
                }

                // Check record type
                if (!/^[A-Z]{1,5}$/.test(record.type)) {
                    validation = {
                        status: false,
                        error: record.type,
                    };
                }

                query = validation.status
                    ? shelljs.exec(command, {
                          silent: true,
                      })
                    : {
                          stdout: `Input validation failed\n${validation.error}\n`,
                          stderr: '',
                          code: 999,
                      };

                // Query result
                const result = {
                    server: {
                        name: server.name,
                        address: server.address,
                    },
                    result: {
                        values: query.stdout.split('\n'),
                        error: query.stderr,
                        code: query.code,
                    },
                    command: command,
                };

                // Check if this is different from other resolves (row level)
                rs.results.forEach(function(previous) {
                    // Compare previous value
                    let compare1 = JSON.stringify(
                        previous.result.values.sort(array_order)
                    );

                    // Compare resolver value
                    let compare2 = JSON.stringify(
                        query.stdout.split('\n').sort(array_order)
                    );

                    // Compare and set diff
                    if (compare1 !== compare2) rs.diff = true;
                });

                // Add result to record results
                rs.results.push(result);
            });

            // Push record set to data
            data.push(rs);
        });

        // Send data
        res.json({
            success: true,
            data: data,
        });
    } else {
        // Send error
        res.json({
            success: false,
            message: 'Missing POST data',
        });
    }
});

module.exports = router;
