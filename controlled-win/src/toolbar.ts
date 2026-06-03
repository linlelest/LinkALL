import { mount } from 'svelte';
import Toolbar from './Toolbar.svelte';
import './app.css';

mount(Toolbar, { target: document.getElementById('app')! });
