package detector_test

import (
	"slices"
	"testing"
)

// FastAPI Comprehensive Tests
func TestFastAPIComprehensive(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedSignals []string
		packageManager  string
	}{
		{
			name: "FastAPI with Poetry and async endpoints",
			files: map[string]string{
				"main.py": `from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
import uvicorn

app = FastAPI(title="My API", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

class Item(BaseModel):
    name: str
    price: float

@app.get("/")
async def root():
    return {"message": "Hello World"}

@app.get("/health")
async def health_check():
    return {"status": "ok"}

@app.post("/items/")
async def create_item(item: Item):
    return item

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)`,
				"pyproject.toml": `[tool.poetry]
name = "fastapi-app"
version = "0.1.0"
description = "FastAPI application"

[tool.poetry.dependencies]
python = "^3.9"
fastapi = "^0.104.0"
uvicorn = {extras = ["standard"], version = "^0.24.0"}
pydantic = "^2.0.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.0.0"
httpx = "^0.25.0"

[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"`,
				"poetry.lock": `[[package]]
name = "fastapi"
version = "0.104.0"
description = "FastAPI framework, high performance, easy to learn"
category = "main"
optional = false
python-versions = ">=3.7"`,
				"app/routers/users.py": `from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

router = APIRouter(prefix="/users", tags=["users"])

@router.get("/")
async def read_users():
    return [{"username": "test"}]`,
				"app/models.py": `from sqlalchemy import Column, Integer, String
from sqlalchemy.ext.declarative import declarative_base

Base = declarative_base()

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    username = Column(String, unique=True)`,
			},
			expectedSignals: []string{"FastAPI import in main/app file", "FastAPI in dependencies"},
			packageManager:  "poetry",
		},
		{
			name: "FastAPI with pip and requirements.txt",
			files: map[string]string{
				"app.py": `from fastapi import FastAPI
from fastapi.responses import JSONResponse

app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "World"}

@app.get("/items/{item_id}")
def read_item(item_id: int, q: str = None):
    return {"item_id": item_id, "q": q}`,
				"requirements.txt": `fastapi==0.104.0
uvicorn[standard]==0.24.0
pydantic==2.4.0
python-multipart==0.0.6`,
				"tests/test_main.py": `from fastapi.testclient import TestClient
from app import app

client = TestClient(app)

def test_read_main():
    response = client.get("/")
    assert response.status_code == 200
    assert response.json() == {"Hello": "World"}`,
			},
			expectedSignals: []string{"FastAPI import in main/app file", "FastAPI in dependencies"},
			packageManager:  "pip",
		},
		{
			name: "FastAPI with uv package manager",
			files: map[string]string{
				"main.py": `import asyncio
from fastapi import FastAPI, BackgroundTasks
from fastapi.staticfiles import StaticFiles
from contextlib import asynccontextmanager

@asynccontextmanager
async def lifespan(app: FastAPI):
    print("Application startup")
    yield
    print("Application shutdown")

app = FastAPI(lifespan=lifespan)
app.mount("/static", StaticFiles(directory="static"), name="static")

def write_log(message: str):
    with open("log.txt", mode="a") as log:
        log.write(message)

@app.post("/send-notification/")
async def send_notification(email: str, background_tasks: BackgroundTasks):
    background_tasks.add_task(write_log, f"Notification sent to {email}")
    return {"message": "Notification sent"}`,
				"pyproject.toml": `[project]
name = "fastapi-uv-app"
version = "0.1.0"
dependencies = [
    "fastapi>=0.104.0",
    "uvicorn[standard]>=0.24.0",
]

[tool.uv]
dev-dependencies = [
    "pytest>=7.0.0",
    "httpx>=0.25.0",
]`,
				"uv.lock": `version = 1
requires-python = ">=3.8"

[[package]]
name = "fastapi"
version = "0.104.0"`,
			},
			expectedSignals: []string{"FastAPI import in main/app file", "FastAPI in dependencies"},
			packageManager:  "uv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "FastAPI" {
				t.Errorf("Expected FastAPI, got %s", detection.Framework)
			}

			if detection.Language != "Python" {
				t.Errorf("Expected Python, got %s", detection.Language)
			}

			for _, expectedSignal := range tt.expectedSignals {
				if !slices.Contains(detection.Signals, expectedSignal) {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}

			// Verify correct package manager in build plan
			if len(detection.BuildPlan) > 0 {
				firstCommand := detection.BuildPlan[0]
				switch tt.packageManager {
				case "poetry":
					if firstCommand != "poetry install" {
						t.Errorf("Expected poetry install, got %s", firstCommand)
					}
				case "pip":
					if firstCommand != "pip install -r requirements.txt" {
						t.Errorf("Expected pip install -r requirements.txt, got %s", firstCommand)
					}
				case "uv":
					if firstCommand != "uv sync" {
						t.Errorf("Expected uv sync, got %s", firstCommand)
					}
				}
			}

			// Verify health check endpoint
			if detection.Healthcheck != nil {
				if path, ok := detection.Healthcheck["path"].(string); ok {
					if path != "/health" {
						t.Errorf("Expected health check path /health, got %s", path)
					}
				}
			}
		})
	}
}

// NestJS Comprehensive Tests
func TestNestJSComprehensive(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedSignals []string
		packageManager  string
	}{
		{
			name: "NestJS with TypeScript and microservices",
			files: map[string]string{
				"nest-cli.json": `{
  "$schema": "https://json.schemastore.org/nest-cli",
  "collection": "@nestjs/schematics",
  "sourceRoot": "src",
  "compilerOptions": {
    "deleteOutDir": true
  }
}`,
				"package.json": `{
  "name": "nestjs-app",
  "version": "0.0.1",
  "scripts": {
    "build": "nest build",
    "format": "prettier --write \"src/**/*.ts\" \"test/**/*.ts\"",
    "start": "nest start",
    "start:dev": "nest start --watch",
    "start:debug": "nest start --debug --watch",
    "start:prod": "node dist/main"
  },
  "dependencies": {
    "@nestjs/common": "^10.0.0",
    "@nestjs/core": "^10.0.0",
    "@nestjs/platform-express": "^10.0.0",
    "@nestjs/microservices": "^10.0.0",
    "@nestjs/swagger": "^7.0.0",
    "rxjs": "^7.8.1",
    "reflect-metadata": "^0.1.13"
  },
  "devDependencies": {
    "@nestjs/cli": "^10.0.0",
    "@nestjs/schematics": "^10.0.0",
    "@nestjs/testing": "^10.0.0",
    "@types/node": "^20.3.1",
    "typescript": "^5.1.3"
  }
}`,
				"src/main.ts": `import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  const config = new DocumentBuilder()
    .setTitle('API Example')
    .setDescription('The API description')
    .setVersion('1.0')
    .addTag('users')
    .build();
  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api', app, document);

  await app.listen(3000);
}
bootstrap();`,
				"src/app.module.ts": `import { Module } from '@nestjs/common';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { UsersModule } from './users/users.module';

@Module({
  imports: [UsersModule],
  controllers: [AppController],
  providers: [AppService],
})
export class AppModule {}`,
				"src/app.controller.ts": `import { Controller, Get } from '@nestjs/common';
import { AppService } from './app.service';
import { ApiTags, ApiOperation } from '@nestjs/swagger';

@ApiTags('app')
@Controller()
export class AppController {
  constructor(private readonly appService: AppService) {}

  @Get()
  @ApiOperation({ summary: 'Get hello message' })
  getHello(): string {
    return this.appService.getHello();
  }

  @Get('health')
  getHealth() {
    return { status: 'ok', timestamp: new Date().toISOString() };
  }
}`,
				"src/users/users.module.ts": `import { Module } from '@nestjs/common';
import { UsersController } from './users.controller';
import { UsersService } from './users.service';

@Module({
  controllers: [UsersController],
  providers: [UsersService],
})
export class UsersModule {}`,
				"tsconfig.json": `{
  "compilerOptions": {
    "module": "commonjs",
    "declaration": true,
    "removeComments": true,
    "emitDecoratorMetadata": true,
    "experimentalDecorators": true,
    "allowSyntheticDefaultImports": true,
    "target": "ES2021",
    "sourceMap": true,
    "outDir": "./dist",
    "baseUrl": "./",
    "incremental": true,
    "skipLibCheck": true,
    "strictNullChecks": false,
    "noImplicitAny": false,
    "strictBindCallApply": false,
    "forceConsistentCasingInFileNames": false,
    "noFallthroughCasesInSwitch": false
  }
}`,
				"pnpm-lock.yaml": "lockfileVersion: '6.0'",
			},
			expectedSignals: []string{"nest-cli.json", "package.json has @nestjs/core", "NestJS app structure"},
			packageManager:  "pnpm",
		},
		{
			name: "NestJS minimal GraphQL setup",
			files: map[string]string{
				"nest-cli.json": `{"collection": "@nestjs/schematics", "sourceRoot": "src"}`,
				"package.json": `{
  "dependencies": {
    "@nestjs/core": "^10.0.0",
    "@nestjs/graphql": "^12.0.0",
    "@nestjs/apollo": "^12.0.0",
    "apollo-server-express": "^3.12.0",
    "graphql": "^16.8.0"
  }
}`,
				"src/main.ts":        `import { NestFactory } from '@nestjs/core';\nimport { AppModule } from './app.module';`,
				"src/app.module.ts":  `import { Module } from '@nestjs/common';\n@Module({})\nexport class AppModule {}`,
				"src/schema.graphql": `type Query { hello: String }`,
			},
			expectedSignals: []string{"nest-cli.json", "package.json has @nestjs/core", "NestJS app structure"},
			packageManager:  "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "NestJS" {
				t.Errorf("Expected NestJS, got %s", detection.Framework)
			}

			if detection.Language != "TypeScript" {
				t.Errorf("Expected TypeScript, got %s", detection.Language)
			}

			for _, expectedSignal := range tt.expectedSignals {
				if !slices.Contains(detection.Signals, expectedSignal) {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}

			// Verify build plan includes TypeScript compilation
			if len(detection.BuildPlan) >= 2 {
				secondCommand := detection.BuildPlan[1]
				switch tt.packageManager {
				case "pnpm":
					if secondCommand != "pnpm run build" {
						t.Errorf("Expected pnpm run build, got %s", secondCommand)
					}
				case "npm":
					if secondCommand != "npm run build" {
						t.Errorf("Expected npm run build, got %s", secondCommand)
					}
				}
			}
		})
	}
}

// ASP.NET Core Comprehensive Tests
func TestASPNETCoreComprehensive(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedSignals []string
		netVersion      string
	}{
		{
			name: "ASP.NET Core 8 Web API with minimal APIs",
			files: map[string]string{
				"MyWebApi.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.EntityFrameworkCore.InMemory" Version="8.0.0" />
    <PackageReference Include="Microsoft.EntityFrameworkCore.SqlServer" Version="8.0.0" />
    <PackageReference Include="Swashbuckle.AspNetCore" Version="6.5.0" />
  </ItemGroup>

</Project>`,
				"Program.cs": `using Microsoft.EntityFrameworkCore;

var builder = WebApplication.CreateBuilder(args);

// Add services to the container.
builder.Services.AddDbContext<TodoDb>(opt => opt.UseInMemoryDatabase("TodoList"));
builder.Services.AddDatabaseDeveloperPageExceptionFilter();

// Learn more about configuring Swagger/OpenAPI at https://aka.ms/aspnetcore/swashbuckle
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

var app = builder.Build();

// Configure the HTTP request pipeline.
if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseHttpsRedirection();

app.MapGet("/", () => "Hello World!");

app.MapGet("/health", () => new { Status = "Healthy", Timestamp = DateTime.UtcNow });

app.MapGet("/todoitems", async (TodoDb db) =>
    await db.Todos.ToListAsync());

app.MapPost("/todoitems", async (Todo todo, TodoDb db) =>
{
    db.Todos.Add(todo);
    await db.SaveChangesAsync();
    return Results.Created($"/todoitems/{todo.Id}", todo);
});

app.Run();

class Todo
{
    public int Id { get; set; }
    public string? Name { get; set; }
    public bool IsComplete { get; set; }
}

class TodoDb : DbContext
{
    public TodoDb(DbContextOptions<TodoDb> options) : base(options) { }
    public DbSet<Todo> Todos => Set<Todo>();
}`,
				"appsettings.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "AllowedHosts": "*",
  "ConnectionStrings": {
    "DefaultConnection": "Server=(localdb)\\mssqllocaldb;Database=TodoDb;Trusted_Connection=true;MultipleActiveResultSets=true"
  }
}`,
				"appsettings.Development.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  }
}`,
				"Properties/launchSettings.json": `{
  "profiles": {
    "http": {
      "commandName": "Project",
      "dotnetRunMessages": true,
      "launchBrowser": true,
      "launchUrl": "swagger",
      "applicationUrl": "http://localhost:5000",
      "environmentVariables": {
        "ASPNETCORE_ENVIRONMENT": "Development"
      }
    }
  }
}`,
			},
			expectedSignals: []string{".csproj file", "ASP.NET Core entry point", "appsettings.json"},
			netVersion:      "net8.0",
		},
		{
			name: "ASP.NET Core MVC with controllers",
			files: map[string]string{
				"WebApp.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
  </PropertyGroup>
</Project>`,
				"Program.cs": `var builder = WebApplication.CreateBuilder(args);

// Add services to the container.
builder.Services.AddControllersWithViews();

var app = builder.Build();

// Configure the HTTP request pipeline.
if (!app.Environment.IsDevelopment())
{
    app.UseExceptionHandler("/Home/Error");
    app.UseHsts();
}

app.UseHttpsRedirection();
app.UseStaticFiles();
app.UseRouting();
app.UseAuthorization();

app.MapControllerRoute(
    name: "default",
    pattern: "{controller=Home}/{action=Index}/{id?}");

app.Run();`,
				"Controllers/HomeController.cs": `using Microsoft.AspNetCore.Mvc;

namespace WebApp.Controllers;

public class HomeController : Controller
{
    public IActionResult Index()
    {
        return View();
    }

    public IActionResult Privacy()
    {
        return View();
    }

    [ResponseCache(Duration = 0, Location = ResponseCacheLocation.None, NoStore = true)]
    public IActionResult Error()
    {
        return View();
    }
}`,
				"Controllers/ApiController.cs": `using Microsoft.AspNetCore.Mvc;

namespace WebApp.Controllers;

[ApiController]
[Route("api/[controller]")]
public class ApiController : ControllerBase
{
    [HttpGet]
    public IActionResult Get()
    {
        return Ok(new { message = "Hello from API" });
    }

    [HttpGet("health")]
    public IActionResult Health()
    {
        return Ok(new { status = "healthy" });
    }
}`,
				"Views/Home/Index.cshtml": `@{
    ViewData["Title"] = "Home Page";
}

<div class="text-center">
    <h1 class="display-4">Welcome</h1>
    <p>Learn about <a href="https://docs.microsoft.com/aspnet/core">building Web apps with ASP.NET Core</a>.</p>
</div>`,
				"appsettings.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "AllowedHosts": "*"
}`,
			},
			expectedSignals: []string{".csproj file", "ASP.NET Core entry point", "appsettings.json"},
			netVersion:      "net7.0",
		},
		{
			name: "ASP.NET Core with Startup.cs (legacy style)",
			files: map[string]string{
				"LegacyApp.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.Authentication.JwtBearer" Version="6.0.0" />
  </ItemGroup>
</Project>`,
				"Program.cs": `using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;

namespace LegacyApp
{
    public class Program
    {
        public static void Main(string[] args)
        {
            CreateHostBuilder(args).Build().Run();
        }

        public static IHostBuilder CreateHostBuilder(string[] args) =>
            Host.CreateDefaultBuilder(args)
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    webBuilder.UseStartup<Startup>();
                });
    }
}`,
				"Startup.cs": `using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

namespace LegacyApp
{
    public class Startup
    {
        public Startup(IConfiguration configuration)
        {
            Configuration = configuration;
        }

        public IConfiguration Configuration { get; }

        public void ConfigureServices(IServiceCollection services)
        {
            services.AddControllers();
            services.AddSwaggerGen();
        }

        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            if (env.IsDevelopment())
            {
                app.UseDeveloperExceptionPage();
                app.UseSwagger();
                app.UseSwaggerUI();
            }

            app.UseHttpsRedirection();
            app.UseRouting();
            app.UseAuthorization();
            app.UseEndpoints(endpoints =>
            {
                endpoints.MapControllers();
            });
        }
    }
}`,
				"appsettings.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Information"
    }
  }
}`,
			},
			expectedSignals: []string{".csproj file", "ASP.NET Core entry point", "appsettings.json"},
			netVersion:      "net6.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "ASP.NET Core" {
				t.Errorf("Expected ASP.NET Core, got %s", detection.Framework)
			}

			if detection.Language != "C#" {
				t.Errorf("Expected C#, got %s", detection.Language)
			}

			for _, expectedSignal := range tt.expectedSignals {
				if !slices.Contains(detection.Signals, expectedSignal) {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}

			// Verify dotnet build commands
			if len(detection.BuildPlan) >= 2 {
				if detection.BuildPlan[0] != "dotnet restore" {
					t.Errorf("Expected 'dotnet restore', got %s", detection.BuildPlan[0])
				}
				if detection.BuildPlan[1] != "dotnet publish -c Release -o out" {
					t.Errorf("Expected 'dotnet publish -c Release -o out', got %s", detection.BuildPlan[1])
				}
			}

			// Verify run command uses shell expansion to find dll
			if len(detection.RunPlan) > 0 {
				if detection.RunPlan[0] != "dotnet $(ls out/*.dll | head -n 1)" {
					t.Errorf("Expected 'dotnet $(ls out/*.dll | head -n 1)', got %s", detection.RunPlan[0])
				}
			}
		})
	}
}
