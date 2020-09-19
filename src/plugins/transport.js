const btoa = require('btoa');
const atob = require('atob');

const encode = function(object) {
	// JSON stringify, reverse, base64
	const encoded = btoa(
		JSON.stringify(object)
			.split('')
			.reverse()
			.join('')
	);

	return encoded;
};

const decode = function(string) {
	// base64, reverse, JSON parse
	const decoded = JSON.parse(
		atob(string)
			.split('')
			.reverse()
			.join('')
	);

	return decoded;
};

export { encode, decode };
//module.exports = { encode, decode };
