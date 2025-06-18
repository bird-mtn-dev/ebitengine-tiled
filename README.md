<h1 style='text-align:center'>Ebitengine Tiled</h1>

[![Discord](https://img.shields.io/discord/958140778931175424?style=for-the-badge&labelColor=%23202e3bff&color=%235a7d93ff%20&label=Discord&logo=discord&logoColor=white)](https://discord.gg/ujEeeHgptU)
[![Subreddit](https://img.shields.io/reddit/subreddit-subscribers/birdmtndev?style=for-the-badge&logo=reddit&logoColor=white&label=r%2Fbirdmtndev&labelColor=%23202e3bff&color=%235a7d93ff&cacheSeconds=120)](https://www.reddit.com/r/birdmtndev)

This project is built to parse and render tiled maps

Features
-------
* Ability to use a provided Filesystem.
* Ability to render animated tile maps

Example
------
```golang
    
    // Load the xml output from tiled during the initilization of the Scene.
    // Note that OpenTileMap will attempt to load the associated tilesets and tile images 
	Tilemap = etiled.OpenTileMap("assets/tilemap/base.tmx")
    // Defines the draw parameters of the tilemap tiles
	Tilemap.Zoom = 1

    // Call Update on the Tilemap during the ebitengine Update loop
	Tilemap.Update(nil)

    // Call Draw on the Tileap during the ebitegine Draw loop
    Tilemap.Draw(worldScreen)

```


License
-------

The template is licensed under the [MIT license](https://opensource.org/licenses/MIT).

Maintainers
-------
* Mark Carpenter <mark@bird-mtn.dev>

Contributing
-------
Want to help improve the template? Check out our [current issues](https://github.com/bird-mtn-dev/ebitengine-tiled/issues). 

Want to know the steps on how to start contributing, take a look at the [open source guide](https://opensource.guide/how-to-contribute/).