const order = function(x, y) {
	var pre = ['string', 'number', 'bool'];
	if (typeof x !== typeof y)
		return pre.indexOf(typeof y) - pre.indexOf(typeof x);

	if (x === y) return 0;
	else return x > y ? 1 : -1;
};

export { order };
