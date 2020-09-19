<template>
	<div id="app">
		<nuxt />
	</div>
</template>

<script>
import ProjectFooter from '~/components/Footer.vue';

export default {
	components: {
		ProjectFooter,
	},
	data() {
		return {};
	},
	methods: {
		handleClick(e) {
			console.log('click', e);
		},
		titleClick(e) {
			console.log('titleClick', e);
		},
	},
	watch: {
		openKeys(val) {
			console.log('openKeys', val);
		},
	},
};
</script>

<style>
.logo {
	margin: 0;
	padding: 24px;
	width: 250px;
	min-height: 20px;
}

.logo img {
	height: 30px;
}
</style>
