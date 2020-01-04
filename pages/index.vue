<template>
    <section>
        <div class="container ct-example-row">
            <div class="row">
                <div class="col">
                    <b-form-group
                        id="tableFieldset-1"
                        label="Servers"
                        label-size="lg"
                        label-class="font-weight-bold pt-0"
                    >
                        <b-form
                            inline
                            v-on:submit.prevent
                            @change="settingsChanged()"
                        >
                            <div
                                inline
                                v-for="(server, index) in servers"
                                class="row"
                            >
                                <b-form-checkbox v-model="server.enabled" />
                                <b-form-input v-model="server.name" />
                                <b-form-input v-model="server.address" />
                                <b-button
                                    variant="outline-danger"
                                    @click="removeServer(index)"
                                    >Remove</b-button
                                >
                            </div>
                        </b-form>
                        <b-row>
                            <b-col>
                                <b-button variant="primary" @click="addServer"
                                    >Add server</b-button
                                >
                            </b-col>
                        </b-row>
                    </b-form-group>
                </div>
                <div class="col">
                    <b-form-group
                        id="tableFieldset-1"
                        label="Records"
                        label-size="lg"
                        label-class="font-weight-bold pt-0"
                    >
                        <b-form
                            inline
                            v-on:submit.prevent
                            @change="settingsChanged()"
                        >
                            <div v-for="(record, index) in records" class="row">
                                <b-form-checkbox v-model="record.enabled" />
                                <b-form-input v-model="record.fqdn" />
                                <b-form-select v-model="record.type">
                                    <option
                                        v-for="record in recordTypes"
                                        :value="record.type"
                                        >{{ record.type }}</option
                                    >
                                </b-form-select>
                                <b-button
                                    variant="outline-danger"
                                    @click="removeRecord(index)"
                                    >Remove</b-button
                                >
                            </div>
                        </b-form>
                        <b-row>
                            <b-col>
                                <b-button variant="primary" @click="addRecord"
                                    >Add record</b-button
                                >
                            </b-col>
                        </b-row>
                    </b-form-group>
                </div>
            </div>
            <div class="row">
                <div class="col text-center">
                    <b-button variant="success" size="lg" @click="submitQuery"
                        >Query</b-button
                    >
                </div>
            </div>

            <b-row>
                <b-col>
                    <table class="result">
                        <thead v-if="response[0]">
                            <td></td>
                            <td
                                v-for="(rs, index) in response[0].results"
                                v-bind:key="rs.record"
                            >
                                {{ rs.server.address }}
                            </td>
                        </thead>
                        <tbody>
                            <tr
                                v-for="(rs, index) in response"
                                v-bind:key="index"
                                v-bind:class="[rs.diff ? 'diff' : 'no-diff']"
                            >
                                <td>
                                    {{ rs.record.fqdn }} ({{ rs.record.type }})
                                </td>
                                <td
                                    v-for="(result, index) in rs.results"
                                    v-bind:key="index"
                                >
                                    <pre><template v-for="value in result.result.values"><template v-if="value">{{ value }}
</template></template></pre>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </b-col>
            </b-row>
        </div>
    </section>
</template>

<script>
import axios from '~/plugins/axios';

import { encode, decode } from '../plugins/transport.js';

export default {
    components: {},
    beforeMount() {
        const urlParams = new URLSearchParams(window.location.search);
        const settingsParam = urlParams.get('settings');
        if (settingsParam) {
            const settings = decode(settingsParam);

            console.log('Loaded settings:', settings);

            // Restore settings state
            this.servers = settings.servers;
            this.records = settings.records;
        }
    },
    data() {
        return {
            response: [],
            servers: [
                {
                    enabled: true,
                    name: 'Google DNS 1',
                    address: '8.8.8.8',
                },
                {
                    enabled: true,
                    name: 'Cloudflare',
                    address: '1.1.1.1',
                },
            ],
            records: [
                {
                    enabled: true,
                    fqdn: 'github.com',
                    type: 'NS',
                },
                {
                    enabled: true,
                    fqdn: 'github.com',
                    type: 'MX',
                },
                {
                    enabled: true,
                    fqdn: 'example.org',
                    type: 'NS',
                },
            ],
            recordTypes: [
                {
                    type: 'A',
                    name: '',
                },
                {
                    type: 'AAAA',
                    name: '',
                },
                {
                    type: 'CNAME',
                    name: '',
                },
                {
                    type: 'MX',
                    name: '',
                },
                {
                    type: 'NS',
                    name: '',
                },
                {
                    type: 'PTR',
                    name: '',
                },
                {
                    type: 'SOA',
                    name: '',
                },
                {
                    type: 'DS',
                    name: '',
                },
                {
                    type: 'TXT',
                    name: '',
                },
            ],
        };
    },
    head() {
        return {
            title: 'DNS Propagation Checker',
        };
    },
    watch: {
        servers: function(value) {
            this.settingsChanged();
        },
        records: function(value) {
            this.settingsChanged();
        },
    },
    methods: {
        addRecord() {
            this.records.push({
                enabled: true,
                fqdn: '',
                type: '',
            });
        },
        removeRecord(index) {
            this.records.splice(index, 1);
        },
        addServer() {
            this.servers.push({
                enabled: true,
                name: '',
                address: '',
            });
        },
        removeServer(index) {
            this.servers.splice(index, 1);
        },
        submitQuery() {
            const self = this;

            const payload = {
                data: encode({
                    servers: this.servers,
                    records: this.records,
                }),
            };

            axios
                .post('/api/query', payload)
                .then(function(response) {
                    const results = response.data.data;

                    self.response = results;

                    /*
                    console.log(results);

                    const records = self.records;
                    const servers = self.servers;

                    //let response = this.response;

                    results.forEach(function(rs) {
                        let r =
                        rs.results.forEach(function(result) {
                            let identifer = `${rs.record.type}-${rs.record.fqdn}-${result.server.address}`;
                            self.response[identifer] = {
                                record: rs.record,
                                server: result.server,
                                result: result.result,
                            };

                            console.log('Identifier: ' + identifer);
                            console.log(
                                'Data in identifier',
                                self.response[identifer]
                            );
                        });
                    });

                    console.log('DONE', self.response);
                    */

                    // Update response structure
                    //this.updateResponseStructure();
                })
                .catch(function(error) {
                    console.log(error);
                });
        },
        settingsChanged() {
            this.updateHistory();
            this.updateResponseStructure();
        },
        updateResponseStructure() {
            //const records = this.records;
            //const servers = this.servers;
            //const response = this.response;
        },
        updateHistory() {
            if (!process.server) {
                // payload, reverse, base64 encode
                const settings = encode({
                    servers: this.servers,
                    records: this.records,
                });

                history.replaceState({}, null, '?settings=' + settings);
            }
        },
    },
};
</script>

<style scoped>
.title {
    margin: 0;
    padding: 10px 0;
    font-size: 20px;
    font-weight: bold;
}

/*
.group {
	padding: 5px;
	background-color: #f1f1f1;
}
*/

.row {
    padding: 5px 0 0 0;
}

.form {
}

form input,
form select,
form option,
form b-button {
    margin: 0;
    padding: 4px;
    min-height: 30px;
}
</style>
