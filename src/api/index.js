// Express
const express = require('express');

// Parse body requests
const bodyParser = require('body-parser');

// API routes
const query = require('./routes/query');

// Create express instnace
const app = express();

// Parse application/x-www-form-urlencoded
app.use(bodyParser.urlencoded({ extended: false }));

// Parse application/json
app.use(bodyParser.json());

// Import API Routes
app.use(query);

// Export the server middleware
module.exports = {
	path: '/api',
	handler: app,
};

const find = db.users.findMany({
	include: {
		id: true,
		Item: {
			createdAt: true,
			ItemOnUser: {
				picked: true,
			},
		},
	},
});
